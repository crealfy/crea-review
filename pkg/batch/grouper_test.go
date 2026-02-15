package batch

import (
	"testing"

	"github.com/crealfy/crea-review/pkg/priority"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.MaxFilesPerBatch != 50 {
		t.Errorf("MaxFilesPerBatch = %d, want 50", opts.MaxFilesPerBatch)
	}
	if opts.MaxTotalFiles != 0 {
		t.Errorf("MaxTotalFiles = %d, want 0", opts.MaxTotalFiles)
	}
	if !opts.GroupByPackage {
		t.Error("GroupByPackage = false, want true")
	}
	if !opts.PairTests {
		t.Error("PairTests = false, want true")
	}
}

func TestGrouperEmpty(t *testing.T) {
	grouper := NewGrouper(DefaultOptions())
	batches := grouper.Group(nil)

	if len(batches) != 0 {
		t.Errorf("len(batches) = %d, want 0", len(batches))
	}
}

func TestGrouperSimple(t *testing.T) {
	grouper := NewGrouper(DefaultOptions())
	scores := []priority.Score{
		{Path: "pkg/auth/handler.go", Total: 80},
		{Path: "pkg/auth/service.go", Total: 60},
		{Path: "pkg/models/user.go", Total: 40},
	}

	batches := grouper.Group(scores)

	// Should create at least 2 batches (auth and models packages)
	if len(batches) < 1 {
		t.Errorf("len(batches) = %d, want at least 1", len(batches))
	}

	// Total files should match
	totalFiles := FilesInBatches(batches)
	if totalFiles != 3 {
		t.Errorf("totalFiles = %d, want 3", totalFiles)
	}
}

func TestGrouperTestPairing(t *testing.T) {
	grouper := NewGrouper(DefaultOptions())
	scores := []priority.Score{
		{Path: "pkg/auth/handler.go", Total: 80},
		{Path: "pkg/auth/handler_test.go", Total: 30},
	}

	batches := grouper.Group(scores)

	// Should pair test with source in same batch
	found := false
	for _, batch := range batches {
		hasSource := false
		hasTest := false
		for _, f := range batch.Files {
			if f == "pkg/auth/handler.go" {
				hasSource = true
			}
			if f == "pkg/auth/handler_test.go" {
				hasTest = true
			}
		}
		if hasSource && hasTest {
			found = true
			if batch.Reason != "test-pair" {
				t.Errorf("batch.Reason = %q, want %q", batch.Reason, "test-pair")
			}
		}
	}

	if !found {
		t.Error("source and test files not paired together")
	}
}

func TestGrouperPackageGrouping(t *testing.T) {
	grouper := NewGrouper(DefaultOptions())
	scores := []priority.Score{
		{Path: "pkg/auth/handler.go", Total: 80},
		{Path: "pkg/auth/service.go", Total: 60},
		{Path: "pkg/auth/middleware.go", Total: 50},
	}

	batches := grouper.Group(scores)

	// All files should be in one batch (same package)
	if len(batches) != 1 {
		t.Errorf("len(batches) = %d, want 1 (same package)", len(batches))
	}

	if len(batches[0].Files) != 3 {
		t.Errorf("len(batches[0].Files) = %d, want 3", len(batches[0].Files))
	}
}

func TestGrouperMaxFilesPerBatch(t *testing.T) {
	opts := DefaultOptions()
	opts.MaxFilesPerBatch = 2

	grouper := NewGrouper(opts)
	scores := []priority.Score{
		{Path: "pkg/a/1.go", Total: 50},
		{Path: "pkg/a/2.go", Total: 40},
		{Path: "pkg/a/3.go", Total: 30},
		{Path: "pkg/a/4.go", Total: 20},
	}

	batches := grouper.Group(scores)

	// Should split into multiple batches
	if len(batches) < 2 {
		t.Errorf("len(batches) = %d, want at least 2", len(batches))
	}

	// Each batch should have at most 2 files
	for _, batch := range batches {
		if len(batch.Files) > 2 {
			t.Errorf("batch has %d files, want at most 2", len(batch.Files))
		}
	}
}

func TestGrouperMaxTotalFiles(t *testing.T) {
	opts := DefaultOptions()
	opts.MaxTotalFiles = 3

	grouper := NewGrouper(opts)
	scores := []priority.Score{
		{Path: "pkg/a/1.go", Total: 50},
		{Path: "pkg/a/2.go", Total: 40},
		{Path: "pkg/a/3.go", Total: 30},
		{Path: "pkg/a/4.go", Total: 20},
		{Path: "pkg/a/5.go", Total: 10},
	}

	batches := grouper.Group(scores)

	totalFiles := FilesInBatches(batches)
	if totalFiles > 3 {
		t.Errorf("totalFiles = %d, want at most 3", totalFiles)
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"handler_test.go", true},
		{"auth.test.js", true},
		{"auth.test.ts", true},
		{"auth.spec.js", true},
		{"test_auth.py", true},
		{"auth_test.py", true},
		{"AuthTest.java", true},
		{"AuthTest.kt", true},
		{"handler.go", false},
		{"auth.js", false},
		{"auth.py", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isTestFile(tt.path)
			if got != tt.expected {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestGetSourcePath(t *testing.T) {
	tests := []struct {
		testPath   string
		sourcePath string
	}{
		{"pkg/auth/handler_test.go", "pkg/auth/handler.go"},
		{"src/auth.test.js", "src/auth.js"},
		{"src/auth.spec.ts", "src/auth.ts"},
		{"test_auth.py", "auth.py"},
		{"auth_test.py", "auth.py"},
		{"AuthTest.java", "Auth.java"},
		{"AuthTest.kt", "Auth.kt"},
	}

	for _, tt := range tests {
		t.Run(tt.testPath, func(t *testing.T) {
			got := getSourcePath(tt.testPath)
			if got != tt.sourcePath {
				t.Errorf("getSourcePath(%q) = %q, want %q", tt.testPath, got, tt.sourcePath)
			}
		})
	}
}

func TestSortBatches(t *testing.T) {
	batches := []Batch{
		{ID: 1, TotalScore: 30},
		{ID: 2, TotalScore: 80},
		{ID: 3, TotalScore: 50},
	}

	sortBatches(batches)

	if len(batches) < 3 {
		t.Fatalf("expected 3 batches, got %d", len(batches))
	}

	if batches[0].TotalScore != 80 {
		t.Errorf("batches[0].TotalScore = %f, want 80", batches[0].TotalScore)
	}
	if batches[1].TotalScore != 50 {
		t.Errorf("batches[1].TotalScore = %f, want 50", batches[1].TotalScore)
	}
	if batches[2].TotalScore != 30 {
		t.Errorf("batches[2].TotalScore = %f, want 30", batches[2].TotalScore)
	}
}

func TestFilesInBatches(t *testing.T) {
	batches := []Batch{
		{Files: []string{"a.go", "b.go"}},
		{Files: []string{"c.go"}},
		{Files: []string{"d.go", "e.go", "f.go"}},
	}

	total := FilesInBatches(batches)
	if total != 6 {
		t.Errorf("FilesInBatches() = %d, want 6", total)
	}
}

func TestTakeBatches(t *testing.T) {
	batches := []Batch{
		{ID: 1},
		{ID: 2},
		{ID: 3},
		{ID: 4},
	}

	result := TakeBatches(batches, 2)
	if len(result) != 2 {
		t.Errorf("len(result) = %d, want 2", len(result))
	}

	// Taking more than available returns all
	result = TakeBatches(batches, 10)
	if len(result) != 4 {
		t.Errorf("len(result) = %d, want 4", len(result))
	}
}

func TestTakeFiles(t *testing.T) {
	batches := []Batch{
		{Files: []string{"a.go", "b.go"}},         // 2 files
		{Files: []string{"c.go"}},                 // 1 file (total 3)
		{Files: []string{"d.go", "e.go"}},         // 2 files (total 5)
		{Files: []string{"f.go", "g.go", "h.go"}}, // 3 files (total 8)
	}

	result := TakeFiles(batches, 5)
	// Should take first 3 batches (5 files), not fourth (would be 8)
	if len(result) != 3 {
		t.Errorf("len(result) = %d, want 3", len(result))
	}

	totalFiles := FilesInBatches(result)
	if totalFiles != 5 {
		t.Errorf("totalFiles = %d, want 5", totalFiles)
	}
}

func TestBatchType(t *testing.T) {
	batch := Batch{
		ID:         1,
		Files:      []string{"a.go", "b.go"},
		Reason:     "test-pair",
		TotalScore: 150.5,
	}

	if batch.ID != 1 {
		t.Errorf("ID = %d, want 1", batch.ID)
	}
	if len(batch.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(batch.Files))
	}
	if batch.Reason != "test-pair" {
		t.Errorf("Reason = %q, want %q", batch.Reason, "test-pair")
	}
	if batch.TotalScore != 150.5 {
		t.Errorf("TotalScore = %f, want 150.5", batch.TotalScore)
	}
}
