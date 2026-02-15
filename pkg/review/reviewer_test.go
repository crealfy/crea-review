package review

import (
	"testing"
	"time"

	"github.com/crealfy/crea-review/pkg/context"
	"github.com/crealfy/crea-review/pkg/session"
)

func TestParseFindings(t *testing.T) {
	response := `Here are my findings:

FINDING: [pkg/auth/handler.go:42] [error] [security]
DESCRIPTION: SQL injection vulnerability in user query
FIX: Use parameterized queries instead of string concatenation

FINDING: [pkg/api/routes.go:100] [warning] [performance]
DESCRIPTION: N+1 query detected in loop
FIX: Batch the database queries outside the loop

FINDING: [main.go:15] [suggestion] [style]
DESCRIPTION: Unused import detected
FIX: Remove the unused import
`

	findings := parseFindings(response)

	if len(findings) != 3 {
		t.Fatalf("len(findings) = %d, want 3", len(findings))
	}

	// Check first finding
	f := findings[0]
	if f.File != "pkg/auth/handler.go" {
		t.Errorf("f.File = %q, want %q", f.File, "pkg/auth/handler.go")
	}
	if f.Line != 42 {
		t.Errorf("f.Line = %d, want 42", f.Line)
	}
	if f.Severity != "error" {
		t.Errorf("f.Severity = %q, want %q", f.Severity, "error")
	}
	if f.Category != "security" {
		t.Errorf("f.Category = %q, want %q", f.Category, "security")
	}
	if f.Description != "SQL injection vulnerability in user query" {
		t.Errorf("f.Description = %q, want %q", f.Description, "SQL injection vulnerability in user query")
	}
	if f.SuggestedFix != "Use parameterized queries instead of string concatenation" {
		t.Errorf("f.SuggestedFix = %q", f.SuggestedFix)
	}

	// Check second finding
	f = findings[1]
	if f.File != "pkg/api/routes.go" {
		t.Errorf("f.File = %q, want %q", f.File, "pkg/api/routes.go")
	}
	if f.Severity != "warning" {
		t.Errorf("f.Severity = %q, want %q", f.Severity, "warning")
	}
	if f.Category != "performance" {
		t.Errorf("f.Category = %q, want %q", f.Category, "performance")
	}

	// Check third finding
	f = findings[2]
	if f.Severity != "suggestion" {
		t.Errorf("f.Severity = %q, want %q", f.Severity, "suggestion")
	}
	if f.Category != "style" {
		t.Errorf("f.Category = %q, want %q", f.Category, "style")
	}
}

func TestParseFindingLine(t *testing.T) {
	tests := []struct {
		line     string
		file     string
		lineNum  int
		severity string
		category string
	}{
		{
			line:     "FINDING: [pkg/auth/handler.go:42] [error] [security]",
			file:     "pkg/auth/handler.go",
			lineNum:  42,
			severity: "error",
			category: "security",
		},
		{
			line:     "FINDING: [main.go:10] warning bug",
			file:     "main.go",
			lineNum:  10,
			severity: "warning",
			category: "bug",
		},
		{
			line:     "FINDING: [test.go] suggestion style",
			file:     "test.go",
			lineNum:  0,
			severity: "suggestion",
			category: "style",
		},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			f := parseFindingLine(tt.line)

			if f.File != tt.file {
				t.Errorf("File = %q, want %q", f.File, tt.file)
			}
			if f.Line != tt.lineNum {
				t.Errorf("Line = %d, want %d", f.Line, tt.lineNum)
			}
			if f.Severity != tt.severity {
				t.Errorf("Severity = %q, want %q", f.Severity, tt.severity)
			}
			if f.Category != tt.category {
				t.Errorf("Category = %q, want %q", f.Category, tt.category)
			}
		})
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"42", 42},
		{"100", 100},
		{"0", 0},
		{"123abc", 123},
		{"abc", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseInt(tt.input)
			if got != tt.expected {
				t.Errorf("parseInt(%q) = %d, want %d", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	reviewCtx := &context.ReviewContext{
		RepoPath:   "/test/repo",
		BaseCommit: "abc123",
		HeadCommit: "def456",
		Diff:       "diff --git a/main.go b/main.go\n+ new line",
		ChangedFiles: []context.FileContent{
			{
				Path:     "main.go",
				Language: "go",
				Content:  "package main\n\nfunc main() {}",
				Status:   "modified",
			},
		},
	}

	prompt := buildReviewPrompt(reviewCtx, "Focus on security")

	// Check prompt contains expected sections
	if !contains(prompt, "code reviewer") {
		t.Error("prompt should mention code reviewer")
	}
	if !contains(prompt, "Focus on security") {
		t.Error("prompt should include instructions")
	}
	if !contains(prompt, "## Diff") {
		t.Error("prompt should include diff section")
	}
	if !contains(prompt, "## Changed Files") {
		t.Error("prompt should include changed files section")
	}
	if !contains(prompt, "main.go") {
		t.Error("prompt should include file name")
	}
	if !contains(prompt, "```go") {
		t.Error("prompt should include language-specific code block")
	}
}

func TestBuildReviewPromptWithRelatedFiles(t *testing.T) {
	reviewCtx := &context.ReviewContext{
		RepoPath: "/test/repo",
		Diff:     "diff",
		ChangedFiles: []context.FileContent{
			{Path: "main.go", Language: "go", Content: "code"},
		},
		RelatedFiles: []context.FileContent{
			{
				Path:          "utils.go",
				Language:      "go",
				Content:       "utils code",
				RelatedReason: "co-changed 5 times",
			},
		},
	}

	prompt := buildReviewPrompt(reviewCtx, "")

	if !contains(prompt, "## Related Files") {
		t.Error("prompt should include related files section")
	}
	if !contains(prompt, "co-changed 5 times") {
		t.Error("prompt should include related reason")
	}
}

func TestBuildReviewPromptWithLinterOutput(t *testing.T) {
	reviewCtx := &context.ReviewContext{
		RepoPath:     "/test/repo",
		Diff:         "diff",
		ChangedFiles: []context.FileContent{},
		LinterOutput: []context.LinterFinding{
			{
				Tool:    "golangci-lint",
				File:    "main.go",
				Line:    10,
				Column:  5,
				Level:   "error",
				Message: "unused variable",
			},
		},
	}

	prompt := buildReviewPrompt(reviewCtx, "")

	if !contains(prompt, "## Linter Findings") {
		t.Error("prompt should include linter findings section")
	}
	if !contains(prompt, "golangci-lint") {
		t.Error("prompt should include linter name")
	}
	if !contains(prompt, "unused variable") {
		t.Error("prompt should include linter message")
	}
}

func TestBackendConstants(t *testing.T) {
	if BackendClaude != "claude" {
		t.Errorf("BackendClaude = %q, want %q", BackendClaude, "claude")
	}
	if BackendCodex != "codex" {
		t.Errorf("BackendCodex = %q, want %q", BackendCodex, "codex")
	}
}

func TestResultType(t *testing.T) {
	result := &Result{
		Findings: []session.Finding{
			{
				File:        "main.go",
				Line:        10,
				Severity:    "error",
				Category:    "bug",
				Description: "test",
			},
		},
		RawResponse:  "raw text",
		InputTokens:  100,
		OutputTokens: 200,
		TotalTokens:  300,
		Cost:         0.05,
		Model:        "claude-opus-4-6",
		ExitCode:     0,
		Duration:     5 * time.Second,
	}

	if len(result.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1", len(result.Findings))
	}
	if result.InputTokens != 100 {
		t.Errorf("InputTokens = %d, want 100", result.InputTokens)
	}
	if result.TotalTokens != 300 {
		t.Errorf("TotalTokens = %d, want 300", result.TotalTokens)
	}
	if result.Cost != 0.05 {
		t.Errorf("Cost = %f, want 0.05", result.Cost)
	}
	if result.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", result.Model, "claude-opus-4-6")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
	if result.Duration != 5*time.Second {
		t.Errorf("Duration = %v, want 5s", result.Duration)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

func TestNewReviewer(t *testing.T) {
	tests := []struct {
		name      string
		backend   Backend
		wantError bool
	}{
		{
			name:      "valid claude backend",
			backend:   BackendClaude,
			wantError: false,
		},
		{
			name:      "valid codex backend",
			backend:   BackendCodex,
			wantError: false,
		},
		{
			name:      "invalid backend",
			backend:   Backend("unknown"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := NewReviewer(tt.backend)

			if tt.wantError {
				if err == nil {
					t.Error("expected error for invalid backend")
				}

				return
			}

			// For valid backends, check if it fails gracefully
			// (may fail if actual claude/codex not installed)
			if err != nil {
				// This is expected if the agent binary is not available
				// The error message should indicate availability issue
				if !contains(err.Error(), "not available") {
					t.Logf("NewReviewer() error (agent may not be installed): %v", err)
				}

				return
			}

			if r == nil {
				t.Error("expected reviewer, got nil")
			}

			if r.backend != tt.backend {
				t.Errorf("backend = %q, want %q", r.backend, tt.backend)
			}

			if r.agent == nil {
				t.Error("expected agent to be set")
			}
		})
	}
}

func TestNewReviewerBackendValidation(t *testing.T) {
	// Test that unknown backends return proper error
	_, err := NewReviewer(Backend("openai"))
	if err == nil {
		t.Error("expected error for unknown backend")
	}

	if err != nil {
		if !contains(err.Error(), "unknown backend") {
			t.Errorf("error = %q, want 'unknown backend'", err.Error())
		}
	}
}

func TestOptionsFields(t *testing.T) {
	opts := Options{
		Model:        "claude-opus-4-6",
		Instructions: "Focus on security",
		Env:          map[string]string{"API_KEY": "test"},
		Retries:      3,
		RetryDelayMS: 1000,
	}

	if opts.Model != "claude-opus-4-6" {
		t.Errorf("Model = %q, want %q", opts.Model, "claude-opus-4-6")
	}
	if opts.Instructions != "Focus on security" {
		t.Errorf("Instructions = %q, want %q", opts.Instructions, "Focus on security")
	}
	if opts.Env["API_KEY"] != "test" {
		t.Errorf("Env[\"API_KEY\"] = %q, want %q", opts.Env["API_KEY"], "test")
	}
	if opts.Retries != 3 {
		t.Errorf("Retries = %d, want 3", opts.Retries)
	}
	if opts.RetryDelayMS != 1000 {
		t.Errorf("RetryDelayMS = %d, want 1000", opts.RetryDelayMS)
	}
}

func TestBuildReviewPromptTruncatedFile(t *testing.T) {
	reviewCtx := &context.ReviewContext{
		RepoPath: "/test/repo",
		Diff:     "diff",
		ChangedFiles: []context.FileContent{
			{
				Path:       "large_file.go",
				Language:   "go",
				Content:    "package main",
				Truncated:  true,
				LinesTotal: 5000,
				Status:     "modified",
			},
		},
	}

	prompt := buildReviewPrompt(reviewCtx, "")

	if !contains(prompt, "Truncated to 5000 lines") {
		t.Error("prompt should indicate file truncation")
	}
}

func TestBuildReviewPromptWithInstructions(t *testing.T) {
	instructions := "Focus on:\n1. SQL injection\n2. XSS vulnerabilities"

	reviewCtx := &context.ReviewContext{
		RepoPath: "/test/repo",
		Diff:     "diff",
		ChangedFiles: []context.FileContent{
			{Path: "main.go", Language: "go", Content: "code"},
		},
	}

	prompt := buildReviewPrompt(reviewCtx, instructions)

	if !contains(prompt, "Additional instructions:") {
		t.Error("prompt should include additional instructions header")
	}
	if !contains(prompt, "Focus on:") {
		t.Error("prompt should include custom instructions")
	}
	if !contains(prompt, "SQL injection") {
		t.Error("prompt should include SQL injection instruction")
	}
}
