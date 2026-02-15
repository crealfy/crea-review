package priority

import (
	"context"
	"testing"

	rcontext "github.com/crealfy/crea-review/pkg/context"
)

func TestDefaultWeights(t *testing.T) {
	w := DefaultWeights()

	total := w.LinesChanged + w.Criticality + w.Churn + w.TestCoverage + w.Recency
	if total != 1.0 {
		t.Errorf("weights sum = %f, want 1.0", total)
	}

	if w.LinesChanged != 0.30 {
		t.Errorf("LinesChanged = %f, want 0.30", w.LinesChanged)
	}
	if w.Criticality != 0.25 {
		t.Errorf("Criticality = %f, want 0.25", w.Criticality)
	}
}

func TestIsCriticalPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"pkg/auth/handler.go", true},
		{"pkg/security/validator.go", true},
		{"pkg/crypto/encrypt.go", true},
		{"pkg/api/routes.go", true},
		{"pkg/handler/user.go", true},
		{"pkg/controller/auth.go", true},
		{"pkg/middleware/jwt.go", true},
		{"pkg/payment/stripe.go", true},
		{"pkg/billing/invoice.go", true},
		{"pkg/admin/dashboard.go", true},
		{"pkg/utils/password_helper.go", true},
		{"pkg/config/secret_manager.go", true},
		{"pkg/auth/token_service.go", true},
		{"pkg/store/credential_store.go", true},
		{"pkg/utils/helper.go", false},
		{"pkg/models/user.go", false},
		{"pkg/service/email.go", false},
		{"main.go", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isCriticalPath(tt.path)
			if got != tt.expected {
				t.Errorf("isCriticalPath(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Go tests
		{"pkg/auth/handler_test.go", true},
		{"main_test.go", true},
		// JavaScript/TypeScript tests
		{"src/auth.test.js", true},
		{"src/auth.test.ts", true},
		{"src/auth.test.tsx", true},
		{"src/auth.spec.js", true},
		{"src/auth.spec.ts", true},
		// Python tests
		{"test_auth.py", true},
		{"auth_test.py", true},
		// Java/Kotlin tests
		{"AuthTest.java", true},
		{"AuthTest.kt", true},
		// Non-tests (directory detection requires full path context, not just basename)
		{"pkg/auth/handler.go", false},
		{"src/auth.js", false},
		{"auth.py", false},
		{"main.go", false},
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

func TestHasAssociatedTests(t *testing.T) {
	testFiles := map[string]bool{
		"pkg/auth/handler_test.go": true,
		"src/auth.test.js":         true,
		"test_utils.py":            true,
	}

	tests := []struct {
		path     string
		expected bool
	}{
		{"pkg/auth/handler.go", true},  // has handler_test.go
		{"pkg/auth/service.go", false}, // no service_test.go
		{"src/auth.js", true},          // has auth.test.js
		{"src/other.js", false},
		{"pkg/auth/handler_test.go", true}, // is itself a test file
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := hasAssociatedTests(tt.path, testFiles)
			if got != tt.expected {
				t.Errorf("hasAssociatedTests(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestSortByScore(t *testing.T) {
	scores := []Score{
		{Path: "low.go", Total: 10},
		{Path: "high.go", Total: 90},
		{Path: "medium.go", Total: 50},
	}

	sortByScore(scores)

	if scores[0].Path != "high.go" {
		t.Errorf("scores[0].Path = %q, want %q", scores[0].Path, "high.go")
	}
	if scores[1].Path != "medium.go" {
		t.Errorf("scores[1].Path = %q, want %q", scores[1].Path, "medium.go")
	}
	if scores[2].Path != "low.go" {
		t.Errorf("scores[2].Path = %q, want %q", scores[2].Path, "low.go")
	}
}

func TestScoreTypes(t *testing.T) {
	score := Score{
		Path:           "pkg/auth/handler.go",
		Total:          75.5,
		LinesChanged:   100,
		IsCriticalPath: true,
		ChurnCount:     25,
		HasTests:       true,
		Breakdown: Breakdown{
			LinesChangedScore: 30,
			CriticalityScore:  25,
			ChurnScore:        10,
			TestCoverageScore: 0,
			RecencyScore:      10.5,
		},
	}

	if score.Path != "pkg/auth/handler.go" {
		t.Errorf("Path = %q, want %q", score.Path, "pkg/auth/handler.go")
	}
	if !score.IsCriticalPath {
		t.Error("IsCriticalPath = false, want true")
	}

	// Verify breakdown
	total := score.Breakdown.LinesChangedScore +
		score.Breakdown.CriticalityScore +
		score.Breakdown.ChurnScore +
		score.Breakdown.TestCoverageScore +
		score.Breakdown.RecencyScore

	if total != score.Total {
		t.Errorf("breakdown total = %f, want %f", total, score.Total)
	}
}

func TestNewScorer(t *testing.T) {
	scorer := NewScorer("/test/repo")

	if scorer.repoPath != "/test/repo" {
		t.Errorf("repoPath = %q, want %q", scorer.repoPath, "/test/repo")
	}

	if scorer.weights != DefaultWeights() {
		t.Error("weights not set to default")
	}
}

func TestScorerWithWeights(t *testing.T) {
	customWeights := Weights{
		LinesChanged: 0.5,
		Criticality:  0.2,
		Churn:        0.1,
		TestCoverage: 0.1,
		Recency:      0.1,
	}

	scorer := NewScorer("/test/repo").WithWeights(customWeights)

	if scorer.weights != customWeights {
		t.Error("custom weights not applied")
	}
}

func TestScoreFileBasic(t *testing.T) {
	scorer := NewScorer("/test/repo")
	testFiles := make(map[string]bool)

	file := rcontext.FileContent{
		Path:       "pkg/auth/handler.go",
		LinesAdded: 50,
	}

	// Use context.Background() instead of nil
	ctx := context.Background()
	score := scorer.scoreFile(ctx, file, 100, testFiles)

	// Should have lines changed score
	if score.Breakdown.LinesChangedScore == 0 {
		t.Error("LinesChangedScore should not be 0")
	}

	// Should be critical path
	if !score.IsCriticalPath {
		t.Error("IsCriticalPath should be true for auth path")
	}

	if score.Breakdown.CriticalityScore == 0 {
		t.Error("CriticalityScore should not be 0 for critical path")
	}

	// No tests, should have test coverage score
	if score.Breakdown.TestCoverageScore == 0 {
		t.Error("TestCoverageScore should not be 0 for file without tests")
	}
}

func TestScoreFiles(t *testing.T) {
	tests := []struct {
		name         string
		files        []rcontext.FileContent
		wantCount    int
		verifyScores func([]Score) bool
	}{
		{
			name:      "empty files list",
			files:     []rcontext.FileContent{},
			wantCount: 0,
			verifyScores: func(scores []Score) bool {
				return len(scores) == 0
			},
		},
		{
			name: "single file",
			files: []rcontext.FileContent{
				{Path: "pkg/auth/handler.go", LinesAdded: 50, LinesDeleted: 10},
			},
			wantCount: 1,
			verifyScores: func(scores []Score) bool {
				return len(scores) == 1 &&
					scores[0].Path == "pkg/auth/handler.go" &&
					scores[0].IsCriticalPath
			},
		},
		{
			name: "multiple files with different priorities",
			files: []rcontext.FileContent{
				{Path: "pkg/utils/helper.go", LinesAdded: 10, LinesDeleted: 0},
				{Path: "pkg/auth/handler.go", LinesAdded: 50, LinesDeleted: 10},
				{Path: "main.go", LinesAdded: 5, LinesDeleted: 5},
			},
			wantCount: 3,
			verifyScores: func(scores []Score) bool {
				// Should be sorted by score descending
				// auth handler should be first (critical path + most changes)
				if scores[0].Path != "pkg/auth/handler.go" {
					return false
				}
				// utils should have lower priority
				return scores[0].Total >= scores[1].Total &&
					scores[1].Total >= scores[2].Total
			},
		},
		{
			name: "files with tests",
			files: []rcontext.FileContent{
				{Path: "pkg/auth/handler.go", LinesAdded: 20, LinesDeleted: 0},
				{Path: "pkg/auth/handler_test.go", LinesAdded: 30, LinesDeleted: 0},
				{Path: "pkg/utils/helper.go", LinesAdded: 10, LinesDeleted: 0},
			},
			wantCount: 3,
			verifyScores: func(scores []Score) bool {
				// handler should have tests (because handler_test.go exists)
				handlerScore := findScore(scores, "pkg/auth/handler.go")
				handlerTestScore := findScore(scores, "pkg/auth/handler_test.go")
				utilsScore := findScore(scores, "pkg/utils/helper.go")

				return handlerScore != nil && handlerScore.HasTests &&
					handlerTestScore != nil &&
					utilsScore != nil && !utilsScore.HasTests
			},
		},
		{
			name: "binary files should have zero lines",
			files: []rcontext.FileContent{
				{Path: "image.png", LinesAdded: 0, LinesDeleted: 0},
			},
			wantCount: 1,
			verifyScores: func(scores []Score) bool {
				return len(scores) == 1 && scores[0].LinesChanged == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scorer := NewScorer("/nonexistent/repo")
			scores, err := scorer.ScoreFiles(context.Background(), tt.files)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(scores) != tt.wantCount {
				t.Errorf("got %d scores, want %d", len(scores), tt.wantCount)
			}

			if !tt.verifyScores(scores) {
				t.Error("score verification failed")
			}
		})
	}
}

func findScore(scores []Score, path string) *Score {
	for i := range scores {
		if scores[i].Path == path {
			return &scores[i]
		}
	}

	return nil
}
