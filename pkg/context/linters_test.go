package context

import (
	"context"
	"testing"
)

func TestRunLinter_EmptyCommand(t *testing.T) {
	_, err := RunLinter(context.Background(), LinterOptions{})
	if err == nil {
		t.Error("expected error for empty command")
	}

	if err.Error() != "linter command required" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunLinter_SuccessfulCommand(t *testing.T) {
	// Use echo to simulate a linter that outputs JSON
	findings, err := RunLinter(context.Background(), LinterOptions{
		Command:  `echo '[]'`,
		RepoPath: ".",
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Empty JSON array means no findings
	if findings != nil {
		t.Errorf("expected nil findings, got %v", findings)
	}
}

func TestRunLinter_WithFiles(t *testing.T) {
	// The command should have files appended
	// We use echo to verify the files are passed
	findings, err := RunLinter(context.Background(), LinterOptions{
		Command:  "echo",
		RepoPath: ".",
		Files:    []string{"a.go", "b.go"},
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Output "a.go b.go" becomes a raw finding
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Message != "a.go b.go" {
		t.Errorf("unexpected message: %q", findings[0].Message)
	}
}

func TestRunLinter_AllIgnoresFiles(t *testing.T) {
	findings, err := RunLinter(context.Background(), LinterOptions{
		Command:  "echo 'linting all'",
		RepoPath: ".",
		Files:    []string{"ignored.go"},
		All:      true,
	})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// When All=true, files are ignored
	if len(findings) != 1 || findings[0].Message != "linting all" {
		t.Errorf("unexpected findings: %v", findings)
	}
}

func TestRunLinter_FailedCommand(t *testing.T) {
	_, err := RunLinter(context.Background(), LinterOptions{
		Command:  "exit 1",
		RepoPath: ".",
	})

	if err == nil {
		t.Error("expected error for failed command")
	}
}

func TestRunLinter_FailedWithFindings(t *testing.T) {
	// Some linters exit non-zero when findings exist
	// We should still return findings, not error
	findings, err := RunLinter(context.Background(), LinterOptions{
		Command:  `echo '[{"Tool":"test","Message":"issue"}]' && exit 1`,
		RepoPath: ".",
	})
	if err != nil {
		t.Errorf("should not error when findings exist: %v", err)
	}

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestParseOutput_JSONArray(t *testing.T) {
	input := []byte(`[{"Tool":"golangci-lint","File":"main.go","Line":10,"Message":"unused var"}]`)

	findings := parseOutput(input, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Tool != "golangci-lint" {
		t.Errorf("Tool = %q, want %q", f.Tool, "golangci-lint")
	}

	if f.File != "main.go" {
		t.Errorf("File = %q, want %q", f.File, "main.go")
	}

	if f.Line != 10 {
		t.Errorf("Line = %d, want %d", f.Line, 10)
	}

	if f.Message != "unused var" {
		t.Errorf("Message = %q, want %q", f.Message, "unused var")
	}
}

func TestParseOutput_NestedIssues(t *testing.T) {
	// golangci-lint style nested JSON
	input := []byte(`{"Issues":[{"Tool":"staticcheck","File":"lib.go","Line":5,"Message":"ineffective"}]}`)

	findings := parseOutput(input, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	if findings[0].Tool != "staticcheck" {
		t.Errorf("Tool = %q, want %q", findings[0].Tool, "staticcheck")
	}
}

func TestParseOutput_RawText(t *testing.T) {
	input := []byte("main.go:10: error: something wrong")

	findings := parseOutput(input, nil)

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Tool != "linter" {
		t.Errorf("Tool = %q, want %q", f.Tool, "linter")
	}

	if f.Level != "info" {
		t.Errorf("Level = %q, want %q", f.Level, "info")
	}

	if f.Message != "main.go:10: error: something wrong" {
		t.Errorf("Message = %q, want raw output", f.Message)
	}
}

func TestParseOutput_EmptyOutput(t *testing.T) {
	findings := parseOutput(nil, nil)

	if findings != nil {
		t.Errorf("expected nil for empty output, got %v", findings)
	}
}

func TestParseOutput_WhitespaceOnly(t *testing.T) {
	findings := parseOutput([]byte("   \n\t  "), nil)

	if findings != nil {
		t.Errorf("expected nil for whitespace-only output, got %v", findings)
	}
}

func TestParseOutput_StderrIncluded(t *testing.T) {
	findings := parseOutput([]byte("stdout content"), []byte("stderr content"))

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	msg := findings[0].Message
	if msg != "stdout content\nstderr content" {
		t.Errorf("Message = %q, want combined output", msg)
	}
}

func TestParseOutput_EmptyJSONArray(t *testing.T) {
	findings := parseOutput([]byte("[]"), nil)

	// Empty array = no findings
	if findings != nil {
		t.Errorf("expected nil for empty JSON array, got %v", findings)
	}
}

func TestLinterOptions_Fields(t *testing.T) {
	opts := LinterOptions{
		Command:  "test-cmd",
		RepoPath: "/repo",
		Files:    []string{"a.go", "b.go"},
		All:      true,
	}

	if opts.Command != "test-cmd" {
		t.Errorf("Command = %q, want %q", opts.Command, "test-cmd")
	}

	if opts.RepoPath != "/repo" {
		t.Errorf("RepoPath = %q, want %q", opts.RepoPath, "/repo")
	}

	if len(opts.Files) != 2 {
		t.Errorf("Files len = %d, want %d", len(opts.Files), 2)
	}

	if !opts.All {
		t.Error("All = false, want true")
	}
}
