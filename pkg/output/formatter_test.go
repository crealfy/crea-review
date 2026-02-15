package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/crealfy/crea-review/pkg/review"
	"github.com/crealfy/crea-review/pkg/session"
)

func TestFormatJSON(t *testing.T) {
	formatter := NewFormatter(FormatJSON)

	result := &review.Result{
		Findings: []session.Finding{
			{
				File:        "main.go",
				Line:        10,
				Severity:    "error",
				Category:    "bug",
				Description: "test issue",
			},
		},
	}

	sess := &session.Session{
		ID:               1,
		TotalFilesInDiff: 10,
		FilesReviewed:    5,
		FilesRemaining:   5,
	}

	var buf bytes.Buffer
	err := formatter.Format(&buf, result, sess)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	// Parse JSON output
	var output Output
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if output.SessionID != 1 {
		t.Errorf("SessionID = %d, want 1", output.SessionID)
	}
	if output.TotalFiles != 10 {
		t.Errorf("TotalFiles = %d, want 10", output.TotalFiles)
	}
	if len(output.Findings) != 1 {
		t.Errorf("len(Findings) = %d, want 1", len(output.Findings))
	}
}

func TestFormatPlain(t *testing.T) {
	formatter := NewFormatter(FormatPlain)

	result := &review.Result{
		Findings: []session.Finding{
			{
				File:        "main.go",
				Line:        10,
				Severity:    "error",
				Category:    "bug",
				Description: "Null pointer dereference",
			},
		},
	}

	var buf bytes.Buffer
	err := formatter.Format(&buf, result, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	if !contains(output, "Code Review Results") {
		t.Error("output should contain header")
	}
	if !contains(output, "main.go:10") {
		t.Error("output should contain file location")
	}
	if !contains(output, "Null pointer dereference") {
		t.Error("output should contain description")
	}
}

func TestFormatPromptOnly(t *testing.T) {
	formatter := NewFormatter(FormatPromptOnly)

	result := &review.Result{
		Findings: []session.Finding{
			{
				File:         "main.go",
				Line:         10,
				Severity:     "error",
				Category:     "security",
				Description:  "SQL injection",
				SuggestedFix: "Use parameterized queries",
			},
		},
	}

	var buf bytes.Buffer
	err := formatter.Format(&buf, result, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	if !contains(output, "Fix the following") {
		t.Error("output should contain fix instructions")
	}
	if !contains(output, "SQL injection") {
		t.Error("output should contain issue description")
	}
	if !contains(output, "parameterized queries") {
		t.Error("output should contain suggested fix")
	}
}

func TestFormatPromptOnlyNoFindings(t *testing.T) {
	formatter := NewFormatter(FormatPromptOnly)

	result := &review.Result{
		Findings: []session.Finding{},
	}

	var buf bytes.Buffer
	err := formatter.Format(&buf, result, nil)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	output := buf.String()

	if !contains(output, "No issues found") {
		t.Error("output should indicate no issues")
	}
}

func TestBuildSummary(t *testing.T) {
	tests := []struct {
		findings []session.Finding
		expected string
	}{
		{
			findings: nil,
			expected: "No issues found",
		},
		{
			findings: []session.Finding{{Category: "bug"}},
			expected: "Found 1 issue: 1 bug",
		},
		{
			findings: []session.Finding{
				{Category: "bug"},
				{Category: "security"},
				{Category: "security"},
			},
			expected: "Found 3 issues: 1 bug, 2 securitys",
		},
	}

	for _, tt := range tests {
		summary := buildSummary(tt.findings)
		if summary != tt.expected {
			t.Errorf("buildSummary() = %q, want %q", summary, tt.expected)
		}
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		word     string
		count    int
		expected string
	}{
		{"bug", 1, "bug"},
		{"bug", 2, "bugs"},
		{"issue", 0, "issues"},
		{"issue", 1, "issue"},
	}

	for _, tt := range tests {
		got := pluralize(tt.word, tt.count)
		if got != tt.expected {
			t.Errorf("pluralize(%q, %d) = %q, want %q", tt.word, tt.count, got, tt.expected)
		}
	}
}

func TestSeverityIcon(t *testing.T) {
	// With colors
	formatter := NewFormatter(FormatPlain)

	if icon := formatter.severityIcon("error"); icon != "❌" {
		t.Errorf("severityIcon(error) = %q, want ❌", icon)
	}
	if icon := formatter.severityIcon("warning"); icon != "⚠️" {
		t.Errorf("severityIcon(warning) = %q, want ⚠️", icon)
	}
	if icon := formatter.severityIcon("suggestion"); icon != "💡" {
		t.Errorf("severityIcon(suggestion) = %q, want 💡", icon)
	}

	// Without colors
	formatterNoColor := NewFormatter(FormatPlain).WithNoColor()

	if icon := formatterNoColor.severityIcon("error"); icon != "[X]" {
		t.Errorf("severityIcon(error) no-color = %q, want [X]", icon)
	}
}

func TestBuildImplementationPrompt(t *testing.T) {
	findings := []session.Finding{
		{
			File:         "main.go",
			Line:         10,
			Category:     "security",
			Description:  "SQL injection vulnerability",
			SuggestedFix: "Use parameterized queries",
		},
	}

	prompt := buildImplementationPrompt(findings)

	if !contains(prompt, "Fix the following") {
		t.Error("prompt should contain fix instructions")
	}
	if !contains(prompt, "[SECURITY]") {
		t.Error("prompt should contain category")
	}
	if !contains(prompt, "main.go:10") {
		t.Error("prompt should contain file location")
	}
	if !contains(prompt, "SQL injection") {
		t.Error("prompt should contain description")
	}
	if !contains(prompt, "parameterized queries") {
		t.Error("prompt should contain suggested fix")
	}
}

func TestFormatSessionList(t *testing.T) {
	sessions := []*session.Session{
		{
			ID:             1,
			CreatedAt:      time.Date(2026, 2, 11, 10, 30, 0, 0, time.UTC),
			BaseCommit:     "abc1234567890",
			FilesReviewed:  50,
			FilesRemaining: 0,
			Status:         session.StatusCompleted,
			Findings:       []session.Finding{{}, {}},
		},
		{
			ID:             2,
			CreatedAt:      time.Date(2026, 2, 11, 11, 0, 0, 0, time.UTC),
			BaseCommit:     "def4567890123",
			FilesReviewed:  50,
			FilesRemaining: 47,
			Status:         session.StatusInProgress,
			ContinuedFrom:  1,
			Findings:       []session.Finding{{}},
		},
	}

	var buf bytes.Buffer
	err := FormatSessionList(&buf, sessions)
	if err != nil {
		t.Fatalf("FormatSessionList() error = %v", err)
	}

	output := buf.String()

	if !contains(output, "Review Sessions") {
		t.Error("output should contain header")
	}
	if !contains(output, "Session 1") {
		t.Error("output should contain session 1")
	}
	if !contains(output, "Session 2") {
		t.Error("output should contain session 2")
	}
	if !contains(output, "continued from session 1") {
		t.Error("output should show continuation")
	}
	if !contains(output, "47 remaining") {
		t.Error("output should show remaining files")
	}
}

func TestFormatSessionListEmpty(t *testing.T) {
	var buf bytes.Buffer
	err := FormatSessionList(&buf, nil)
	if err != nil {
		t.Fatalf("FormatSessionList() error = %v", err)
	}

	output := buf.String()

	if !contains(output, "No review sessions found") {
		t.Error("output should indicate no sessions")
	}
}

func TestFormatConstants(t *testing.T) {
	if FormatJSON != "json" {
		t.Errorf("FormatJSON = %q, want %q", FormatJSON, "json")
	}
	if FormatPlain != "plain" {
		t.Errorf("FormatPlain = %q, want %q", FormatPlain, "plain")
	}
	if FormatPromptOnly != "prompt-only" {
		t.Errorf("FormatPromptOnly = %q, want %q", FormatPromptOnly, "prompt-only")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
