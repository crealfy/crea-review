// Package output provides output formatting for review results.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/crealfy/crea-review/pkg/review"
	"github.com/crealfy/crea-review/pkg/session"
)

// Format represents the output format.
type Format string

const (
	FormatJSON       Format = "json"
	FormatPlain      Format = "plain"
	FormatPromptOnly Format = "prompt-only"
)

// Output represents the formatted review output.
type Output struct {
	// SessionID is the session identifier.
	SessionID int `json:"session_id"`

	// TotalFiles is the total files in the diff.
	TotalFiles int `json:"total_files"`

	// ReviewedFiles is the number of files reviewed.
	ReviewedFiles int `json:"reviewed_files"`

	// RemainingFiles is the number of files remaining.
	RemainingFiles int `json:"remaining_files,omitempty"`

	// Summary is a human-readable summary.
	Summary string `json:"summary"`

	// Findings contains the review findings.
	Findings []session.Finding `json:"findings"`

	// ImplementationPrompt is the prompt for crea-pipe to fix issues.
	ImplementationPrompt string `json:"implementation_prompt,omitempty"`

	// Cost is the estimated cost in USD.
	Cost float64 `json:"cost,omitempty"`

	// Model is the model that was used.
	Model string `json:"model,omitempty"`

	// TokenUsage is a formatted token usage string.
	TokenUsage string `json:"token_usage,omitempty"`
}

// Formatter formats review results.
type Formatter struct {
	format  Format
	noColor bool
}

// NewFormatter creates a new formatter.
func NewFormatter(format Format) *Formatter {
	return &Formatter{
		format: format,
	}
}

// WithNoColor disables colored output.
func (f *Formatter) WithNoColor() *Formatter {
	f.noColor = true

	return f
}

// Format formats the review result and writes to the writer.
func (f *Formatter) Format(w io.Writer, result *review.Result, sess *session.Session) error {
	output := f.buildOutput(result, sess)

	switch f.format {
	case FormatJSON:
		return f.formatJSON(w, output)
	case FormatPlain:
		return f.formatPlain(w, output)
	case FormatPromptOnly:
		return f.formatPromptOnly(w, output)
	default:
		return f.formatJSON(w, output)
	}
}

// buildOutput creates the output structure from the result.
func (f *Formatter) buildOutput(result *review.Result, sess *session.Session) *Output {
	output := &Output{
		Findings: result.Findings,
		Summary:  buildSummary(result.Findings),
		Cost:     result.Cost,
		Model:    result.Model,
	}

	// Build token usage string
	if result.InputTokens > 0 || result.OutputTokens > 0 {
		output.TokenUsage = fmt.Sprintf("%d in / %d out (%d total)",
			result.InputTokens, result.OutputTokens, result.TotalTokens)
	}

	if sess != nil {
		output.SessionID = sess.ID
		output.TotalFiles = sess.TotalFilesInDiff
		output.ReviewedFiles = sess.FilesReviewed
		output.RemainingFiles = sess.FilesRemaining
	}

	// Build implementation prompt
	if len(result.Findings) > 0 {
		output.ImplementationPrompt = buildImplementationPrompt(result.Findings)
	}

	return output
}

// formatJSON writes JSON output.
func (f *Formatter) formatJSON(w io.Writer, output *Output) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(output)
}

// formatPlain writes human-readable output.
func (f *Formatter) formatPlain(w io.Writer, output *Output) error {
	var sb strings.Builder

	// Header
	sb.WriteString("Code Review Results\n")
	sb.WriteString("===================\n\n")

	if output.SessionID > 0 {
		sb.WriteString(fmt.Sprintf("Session: %d\n", output.SessionID))
		sb.WriteString(fmt.Sprintf("Files reviewed: %d/%d\n", output.ReviewedFiles, output.TotalFiles))

		if output.RemainingFiles > 0 {
			sb.WriteString(fmt.Sprintf("Files remaining: %d\n", output.RemainingFiles))
		}

		sb.WriteString("\n")
	}

	// Summary
	sb.WriteString("Summary: ")
	sb.WriteString(output.Summary)
	sb.WriteString("\n\n")

	// Findings
	if len(output.Findings) == 0 {
		sb.WriteString("No issues found.\n")
	} else {
		sb.WriteString("Findings\n")
		sb.WriteString("--------\n\n")

		for i, finding := range output.Findings {
			// Severity indicator
			severityIcon := f.severityIcon(finding.Severity)
			sb.WriteString(fmt.Sprintf("%d. %s [%s] %s\n",
				i+1, severityIcon, finding.Severity, finding.Category))
			sb.WriteString(fmt.Sprintf("   File: %s:%d\n", finding.File, finding.Line))
			sb.WriteString(fmt.Sprintf("   %s\n", finding.Description))

			if finding.SuggestedFix != "" {
				sb.WriteString(fmt.Sprintf("   Fix: %s\n", finding.SuggestedFix))
			}

			sb.WriteString("\n")
		}
	}

	_, err := w.Write([]byte(sb.String()))

	return err
}

// formatPromptOnly writes minimal output for piping to crea-pipe.
func (f *Formatter) formatPromptOnly(w io.Writer, output *Output) error {
	if output.ImplementationPrompt == "" {
		_, err := fmt.Fprintln(w, "No issues found in code review.")

		return err
	}

	_, err := w.Write([]byte(output.ImplementationPrompt))

	return err
}

// severityIcon returns an icon for the severity level.
func (f *Formatter) severityIcon(severity string) string {
	if f.noColor {
		switch severity {
		case "error":
			return "[X]"
		case "warning":
			return "[!]"
		default:
			return "[i]"
		}
	}

	switch severity {
	case "error":
		return "❌"
	case "warning":
		return "⚠️"
	default:
		return "💡"
	}
}

// buildSummary creates a human-readable summary of findings.
func buildSummary(findings []session.Finding) string {
	if len(findings) == 0 {
		return "No issues found"
	}

	counts := make(map[string]int)
	for _, f := range findings {
		counts[f.Category]++
	}

	var parts []string
	categories := []string{"bug", "security", "performance", "style", "testing"}

	for _, cat := range categories {
		if count := counts[cat]; count > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", count, pluralize(cat, count)))
		}
	}

	total := len(findings)

	return fmt.Sprintf("Found %d %s: %s",
		total,
		pluralize("issue", total),
		strings.Join(parts, ", "))
}

// pluralize adds 's' for plural.
func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}

	return word + "s"
}

// buildImplementationPrompt creates a prompt for crea-pipe to fix issues.
func buildImplementationPrompt(findings []session.Finding) string {
	var sb strings.Builder

	sb.WriteString("Fix the following code review issues:\n\n")

	for i, f := range findings {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s:%d\n",
			i+1, strings.ToUpper(f.Category), f.File, f.Line))
		sb.WriteString(fmt.Sprintf("   Issue: %s\n", f.Description))

		if f.SuggestedFix != "" {
			sb.WriteString(fmt.Sprintf("   Fix: %s\n", f.SuggestedFix))
		}

		sb.WriteString("\n")
	}

	sb.WriteString("Apply the fixes while maintaining code style and ensuring tests still pass.\n")

	return sb.String()
}

// FormatSessionList formats a list of sessions.
func FormatSessionList(w io.Writer, sessions []*session.Session) error {
	if len(sessions) == 0 {
		_, err := fmt.Fprintln(w, "No review sessions found.")

		return err
	}

	if _, err := fmt.Fprintln(w, "Review Sessions"); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "==============="); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	for _, sess := range sessions {
		status := string(sess.Status)
		if sess.FilesRemaining > 0 {
			status = fmt.Sprintf("%s (%d remaining)", status, sess.FilesRemaining)
		}

		baseCommit := sess.BaseCommit
		if len(baseCommit) > 7 {
			baseCommit = baseCommit[:7]
		}

		if _, err := fmt.Fprintf(w, "Session %d: %d files, %s, base=%s\n",
			sess.ID, sess.FilesReviewed, sess.CreatedAt.Format("2006-01-02 15:04"), baseCommit); err != nil {
			return err
		}

		if sess.ContinuedFrom > 0 {
			if _, err := fmt.Fprintf(w, "  └─ continued from session %d\n", sess.ContinuedFrom); err != nil {
				return err
			}
		}

		if _, err := fmt.Fprintf(w, "  └─ status: %s, findings: %d\n", status, len(sess.Findings)); err != nil {
			return err
		}

		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	return nil
}
