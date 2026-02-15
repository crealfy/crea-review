package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crealfy/crea-pipe/pkg/git"
)

func TestDefaultGatherOptions(t *testing.T) {
	opts := DefaultGatherOptions()

	if opts.HeadCommit != "HEAD" {
		t.Errorf("HeadCommit = %q, want %q", opts.HeadCommit, "HEAD")
	}
	if opts.ReviewType != "all" {
		t.Errorf("ReviewType = %q, want %q", opts.ReviewType, "all")
	}
	if opts.MaxFileLines != 500 {
		t.Errorf("MaxFileLines = %d, want %d", opts.MaxFileLines, 500)
	}
	if !opts.IncludeRelated {
		t.Error("IncludeRelated = false, want true")
	}
	if opts.RelatedDepth != 5 {
		t.Errorf("RelatedDepth = %d, want %d", opts.RelatedDepth, 5)
	}
	if opts.MaxFiles != 50 {
		t.Errorf("MaxFiles = %d, want %d", opts.MaxFiles, 50)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"main.go", "go"},
		{"script.py", "python"},
		{"app.js", "javascript"},
		{"component.tsx", "tsx"},
		{"handler.ts", "typescript"},
		{"lib.rs", "rust"},
		{"Main.java", "java"},
		{"app.kt", "kotlin"},
		{"main.c", "c"},
		{"util.cpp", "cpp"},
		{"header.h", "c-header"},
		{"Program.cs", "csharp"},
		{"script.rb", "ruby"},
		{"index.php", "php"},
		{"App.swift", "swift"},
		{"script.sh", "shell"},
		{"query.sql", "sql"},
		{"index.html", "html"},
		{"style.css", "css"},
		{"style.scss", "scss"},
		{"config.json", "json"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"doc.xml", "xml"},
		{"README.md", "markdown"},
		{"schema.proto", "protobuf"},
		{"unknown.xyz", "text"},
		{"Makefile", "text"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectLanguage(tt.path)
			if got != tt.expected {
				t.Errorf("detectLanguage(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestReadFileContent(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, _, _, err := readFileContent("/nonexistent", "file.go", 100, false)
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("success - no truncation", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := "line1\nline2\nline3"
		filePath := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		got, truncated, totalLines, err := readFileContent(tmpDir, "test.go", 10, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != content {
			t.Errorf("content = %q, want %q", got, content)
		}
		if truncated {
			t.Error("expected truncated=false")
		}
		if totalLines != 3 {
			t.Errorf("totalLines = %d, want 3", totalLines)
		}
	})

	t.Run("success - with truncation", func(t *testing.T) {
		tmpDir := t.TempDir()
		lines := make([]string, 10)
		for i := range 10 {
			lines[i] = "line1"
		}
		content := strings.Join(lines, "\n")
		filePath := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		got, truncated, totalLines, err := readFileContent(tmpDir, "test.go", 5, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !truncated {
			t.Error("expected truncated=true")
		}
		if totalLines != 10 {
			t.Errorf("totalLines = %d, want 10", totalLines)
		}
		if !strings.Contains(got, "truncated") {
			t.Error("expected truncation message in content")
		}
		// Check that we got approximately 5 lines plus the truncation message
		gotLines := strings.Split(got, "\n")
		if len(gotLines) > 7 { // 5 lines + 2 for truncation message
			t.Errorf("got %d lines, expected ~6-7", len(gotLines))
		}
	})

	t.Run("noTruncate option", func(t *testing.T) {
		tmpDir := t.TempDir()
		lines := make([]string, 10)
		for i := range 10 {
			lines[i] = "line1"
		}
		content := strings.Join(lines, "\n")
		filePath := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		got, truncated, totalLines, err := readFileContent(tmpDir, "test.go", 5, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if truncated {
			t.Error("expected truncated=false with noTruncate=true")
		}
		if totalLines != 10 {
			t.Errorf("totalLines = %d, want 10", totalLines)
		}
		if got != content {
			t.Error("content should not be truncated with noTruncate=true")
		}
	})

	t.Run("maxLines zero or negative - no truncation", func(t *testing.T) {
		tmpDir := t.TempDir()
		content := "line1\nline2\nline3\n"
		filePath := filepath.Join(tmpDir, "test.go")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		// Test with 0
		got, truncated, _, err := readFileContent(tmpDir, "test.go", 0, false)
		if err != nil {
			t.Fatalf("unexpected error with maxLines=0: %v", err)
		}
		if truncated {
			t.Error("expected truncated=false with maxLines=0")
		}
		if got != content {
			t.Error("content should not be truncated with maxLines=0")
		}
	})
}

func TestReviewContextTypes(t *testing.T) {
	// Test that types can be instantiated correctly
	rc := &ReviewContext{
		RepoPath:   "/test/repo",
		BaseCommit: "abc123",
		HeadCommit: "def456",
		Diff:       "diff content",
		ChangedFiles: []FileContent{
			{
				Path:     "main.go",
				Language: "go",
				Content:  "package main",
			},
		},
		Stats: ReviewStats{
			TotalFiles:    1,
			ReviewedFiles: 1,
		},
	}

	if rc.RepoPath != "/test/repo" {
		t.Errorf("RepoPath = %q, want %q", rc.RepoPath, "/test/repo")
	}
	if len(rc.ChangedFiles) != 1 {
		t.Errorf("len(ChangedFiles) = %d, want 1", len(rc.ChangedFiles))
	}
}

func TestFileContentType(t *testing.T) {
	fc := FileContent{
		Path:          "auth/handler.go",
		Language:      "go",
		Content:       "package auth",
		Truncated:     false,
		LinesTotal:    50,
		LinesAdded:    10,
		LinesDeleted:  5,
		Status:        "modified",
		OldPath:       "",
		IsBinary:      false,
		RelatedReason: "",
	}

	if fc.Path != "auth/handler.go" {
		t.Errorf("Path = %q, want %q", fc.Path, "auth/handler.go")
	}
	if fc.Language != "go" {
		t.Errorf("Language = %q, want %q", fc.Language, "go")
	}
}

func TestLinterFindingType(t *testing.T) {
	finding := LinterFinding{
		Tool:    "golangci-lint",
		File:    "main.go",
		Line:    42,
		Column:  10,
		Level:   "error",
		Message: "unused variable",
		RuleID:  "unused",
	}

	if finding.Tool != "golangci-lint" {
		t.Errorf("Tool = %q, want %q", finding.Tool, "golangci-lint")
	}
	if finding.Line != 42 {
		t.Errorf("Line = %d, want 42", finding.Line)
	}
}

func TestReviewStatsType(t *testing.T) {
	stats := ReviewStats{
		TotalFiles:        100,
		ReviewedFiles:     50,
		SkippedFiles:      45,
		TotalLinesAdded:   500,
		TotalLinesDeleted: 200,
		BinaryFiles:       5,
	}

	if stats.TotalFiles != 100 {
		t.Errorf("TotalFiles = %d, want 100", stats.TotalFiles)
	}
	if stats.SkippedFiles != 45 {
		t.Errorf("SkippedFiles = %d, want 45", stats.SkippedFiles)
	}
}

func TestGatherOptionsType(t *testing.T) {
	opts := GatherOptions{
		BaseCommit:     "abc123",
		HeadCommit:     "HEAD",
		BaseBranch:     "main",
		ReviewType:     "committed",
		MaxFileLines:   1000,
		NoTruncate:     true,
		IncludeRelated: false,
		RelatedDepth:   10,
		IncludeLinters: true,
		MaxFiles:       100,
		ConfigFiles:    []string{"CLAUDE.md", "review.yaml"},
	}

	if opts.BaseCommit != "abc123" {
		t.Errorf("BaseCommit = %q, want %q", opts.BaseCommit, "abc123")
	}
	if !opts.NoTruncate {
		t.Error("NoTruncate = false, want true")
	}
	if len(opts.ConfigFiles) != 2 {
		t.Errorf("len(ConfigFiles) = %d, want 2", len(opts.ConfigFiles))
	}
}

func TestResolveCommits(t *testing.T) {
	tests := []struct {
		name       string
		opts       GatherOptions
		wantBase   string
		wantHead   string
		wantBranch string
		wantError  bool
	}{
		{
			name: "base commit specified",
			opts: GatherOptions{
				BaseCommit: "abc123",
				HeadCommit: "def456",
			},
			wantBase:   "abc123",
			wantHead:   "def456",
			wantBranch: "",
			wantError:  false,
		},
		{
			name: "base commit takes precedence",
			opts: GatherOptions{
				BaseCommit: "abc123",
				HeadCommit: "def456",
				BaseBranch: "main",
				ReviewType: "committed",
			},
			wantBase:   "abc123",
			wantHead:   "def456",
			wantBranch: "",
			wantError:  false,
		},
		{
			name: "review type - uncommitted",
			opts: GatherOptions{
				ReviewType: "uncommitted",
			},
			wantBase:   "HEAD",
			wantHead:   "",
			wantBranch: "",
			wantError:  false,
		},
		{
			name: "review type - committed",
			opts: GatherOptions{
				ReviewType: "committed",
			},
			wantBase:   "HEAD~1",
			wantHead:   "HEAD",
			wantBranch: "",
			wantError:  false,
		},
		{
			name: "review type - all (default)",
			opts: GatherOptions{
				ReviewType: "all",
			},
			wantBase:   "HEAD",
			wantHead:   "",
			wantBranch: "",
			wantError:  false,
		},
		{
			name: "base branch specified - requires git repo",
			opts: GatherOptions{
				BaseBranch: "main",
			},
			wantBase:   "",
			wantHead:   "",
			wantBranch: "",
			wantError:  true, // Will fail because no git repo
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &ReviewContext{
				RepoPath: "/nonexistent/repo",
			}
			err := resolveCommits(context.Background(), rc, tt.opts)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rc.BaseCommit != tt.wantBase {
				t.Errorf("BaseCommit = %q, want %q", rc.BaseCommit, tt.wantBase)
			}
			if rc.HeadCommit != tt.wantHead {
				t.Errorf("HeadCommit = %q, want %q", rc.HeadCommit, tt.wantHead)
			}
			if rc.BaseBranch != tt.wantBranch {
				t.Errorf("BaseBranch = %q, want %q", rc.BaseBranch, tt.wantBranch)
			}
		})
	}
}

func TestGatherFileContents(t *testing.T) {
	t.Run("binary files are skipped", func(t *testing.T) {
		diffFiles := []git.DiffFile{
			{Path: "main.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 5, Status: git.FileModified},
			{Path: "image.png", IsBinary: true, LinesAdded: 0, LinesDeleted: 0, Status: git.FileAdded},
			{Path: "data.bin", IsBinary: true, LinesAdded: 0, LinesDeleted: 0, Status: git.FileModified},
		}

		files, stats := gatherFileContents(context.Background(), "/tmp", diffFiles, GatherOptions{})

		if len(files) != 1 {
			t.Errorf("got %d files, want 1 (binary files skipped)", len(files))
		}
		if stats.BinaryFiles != 2 {
			t.Errorf("BinaryFiles = %d, want 2", stats.BinaryFiles)
		}
		if stats.TotalFiles != 3 {
			t.Errorf("TotalFiles = %d, want 3", stats.TotalFiles)
		}
	})

	t.Run("max files limit", func(t *testing.T) {
		diffFiles := []git.DiffFile{
			{Path: "a.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 0, Status: git.FileModified},
			{Path: "b.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 0, Status: git.FileModified},
			{Path: "c.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 0, Status: git.FileModified},
		}

		opts := GatherOptions{MaxFiles: 2}
		files, stats := gatherFileContents(context.Background(), "/tmp", diffFiles, opts)

		if len(files) != 2 {
			t.Errorf("got %d files, want 2 (max files limit)", len(files))
		}
		if stats.SkippedFiles != 1 {
			t.Errorf("SkippedFiles = %d, want 1", stats.SkippedFiles)
		}
	})

	t.Run("lines counted correctly", func(t *testing.T) {
		diffFiles := []git.DiffFile{
			{Path: "a.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 5, Status: git.FileModified},
			{Path: "b.go", IsBinary: false, LinesAdded: 20, LinesDeleted: 3, Status: git.FileModified},
		}

		_, stats := gatherFileContents(context.Background(), "/tmp", diffFiles, GatherOptions{})

		if stats.TotalLinesAdded != 30 {
			t.Errorf("TotalLinesAdded = %d, want 30", stats.TotalLinesAdded)
		}
		if stats.TotalLinesDeleted != 8 {
			t.Errorf("TotalLinesDeleted = %d, want 8", stats.TotalLinesDeleted)
		}
	})

	t.Run("language detection", func(t *testing.T) {
		diffFiles := []git.DiffFile{
			{Path: "main.go", IsBinary: false, LinesAdded: 1, LinesDeleted: 0, Status: git.FileModified},
			{Path: "script.py", IsBinary: false, LinesAdded: 1, LinesDeleted: 0, Status: git.FileModified},
			{Path: "app.js", IsBinary: false, LinesAdded: 1, LinesDeleted: 0, Status: git.FileModified},
		}

		files, _ := gatherFileContents(context.Background(), "/tmp", diffFiles, GatherOptions{})

		if len(files) != 3 {
			t.Fatalf("got %d files, want 3", len(files))
		}
		if files[0].Language != "go" {
			t.Errorf("files[0].Language = %q, want 'go'", files[0].Language)
		}
		if files[1].Language != "python" {
			t.Errorf("files[1].Language = %q, want 'python'", files[1].Language)
		}
		if files[2].Language != "javascript" {
			t.Errorf("files[2].Language = %q, want 'javascript'", files[2].Language)
		}
	})

	t.Run("file status mapping", func(t *testing.T) {
		diffFiles := []git.DiffFile{
			{Path: "added.go", IsBinary: false, LinesAdded: 10, LinesDeleted: 0, Status: git.FileAdded},
			{Path: "modified.go", IsBinary: false, LinesAdded: 5, LinesDeleted: 3, Status: git.FileModified},
			{Path: "deleted.go", IsBinary: false, LinesAdded: 0, LinesDeleted: 10, Status: git.FileDeleted},
		}

		files, _ := gatherFileContents(context.Background(), "/tmp", diffFiles, GatherOptions{})

		if len(files) != 3 {
			t.Fatalf("got %d files, want 3", len(files))
		}
		if files[0].Status != "added" {
			t.Errorf("files[0].Status = %q, want 'added'", files[0].Status)
		}
		if files[1].Status != "modified" {
			t.Errorf("files[1].Status = %q, want 'modified'", files[1].Status)
		}
		if files[2].Status != "deleted" {
			t.Errorf("files[2].Status = %q, want 'deleted'", files[2].Status)
		}
	})
}
