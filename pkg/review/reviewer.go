// Package review provides AI-powered code review execution.
package review

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/crealfy/crea-pipe/pkg/agent"
	"github.com/crealfy/crea-pipe/pkg/agent/claude"
	"github.com/crealfy/crea-pipe/pkg/agent/codex"
	rcontext "github.com/crealfy/crea-review/pkg/context"
	"github.com/crealfy/crea-review/pkg/session"
)

// Backend represents the AI backend to use.
type Backend string

const (
	BackendClaude Backend = "claude"
	BackendCodex  Backend = "codex"
)

// Reviewer performs AI code reviews.
type Reviewer struct {
	agent   agent.Agent
	backend Backend
}

// NewReviewer creates a new reviewer with the specified backend.
func NewReviewer(backend Backend) (*Reviewer, error) {
	var a agent.Agent

	switch backend {
	case BackendClaude:
		a = claude.NewSDK()
	case BackendCodex:
		a = codex.New()
	default:
		return nil, fmt.Errorf("unknown backend: %s", backend)
	}

	if err := a.Available(); err != nil {
		return nil, fmt.Errorf("%s not available: %w", backend, err)
	}

	return &Reviewer{
		agent:   a,
		backend: backend,
	}, nil
}

// Options configures the review behavior.
type Options struct {
	// Model overrides the default model.
	Model string

	// Instructions are additional review instructions.
	Instructions string

	// StreamHandler receives events during execution.
	StreamHandler func(agent.Event)

	// Env contains environment variables to pass to the agent.
	Env map[string]string

	// Retries is the number of retries on transient failures (0 = no retries).
	Retries int

	// RetryDelayMS is the delay between retries in milliseconds.
	RetryDelayMS int
}

// Review performs a code review on the given context.
func (r *Reviewer) Review(ctx context.Context, reviewCtx *rcontext.ReviewContext, opts Options) (*Result, error) {
	prompt := buildReviewPrompt(reviewCtx, opts.Instructions)

	agentOpts := []agent.Option{
		agent.WithWorkDir(reviewCtx.RepoPath),
	}

	if opts.Model != "" {
		agentOpts = append(agentOpts, agent.WithModel(opts.Model))
	}

	if opts.StreamHandler != nil {
		agentOpts = append(agentOpts, agent.WithStreaming(opts.StreamHandler))
	}

	for k, v := range opts.Env {
		agentOpts = append(agentOpts, agent.WithEnv(k, v))
	}

	if opts.Retries > 0 {
		agentOpts = append(agentOpts, agent.WithRetries(opts.Retries, opts.RetryDelayMS))
	}

	response, err := r.agent.Run(ctx, prompt, agentOpts...)
	if err != nil {
		return nil, fmt.Errorf("run agent: %w", err)
	}

	findings := parseFindings(response.Text)

	return &Result{
		Findings:     findings,
		RawResponse:  response.Text,
		InputTokens:  response.InputTokens,
		OutputTokens: response.OutputTokens,
		TotalTokens:  response.TotalTokens,
		Cost:         response.Cost,
		Model:        response.Model,
		ExitCode:     response.ExitCode,
		Duration:     response.Duration,
	}, nil
}

// Result contains the review results.
type Result struct {
	// Findings contains the parsed review findings.
	Findings []session.Finding

	// RawResponse is the raw AI response text.
	RawResponse string

	// InputTokens is the number of input tokens used.
	InputTokens int

	// OutputTokens is the number of output tokens generated.
	OutputTokens int

	// TotalTokens is the total tokens used (input + output).
	TotalTokens int

	// Cost is the estimated cost in USD.
	Cost float64

	// Model is the model that was used.
	Model string

	// ExitCode is the process exit code (0 for success).
	ExitCode int

	// Duration is how long the review took.
	Duration time.Duration
}

// buildReviewPrompt builds the review prompt from context.
// Keeps it minimal - Claude can read files itself.
func buildReviewPrompt(reviewCtx *rcontext.ReviewContext, instructions string) string {
	var sb strings.Builder

	sb.WriteString("Review the following code changes:\n\n")

	if instructions != "" {
		sb.WriteString("Instructions: ")
		sb.WriteString(instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Changed Files\n\n")

	for _, f := range reviewCtx.ChangedFiles {
		sb.WriteString(fmt.Sprintf("- %s (%s, +%d/-%d lines)\n",
			f.Path, f.Status, f.LinesAdded, f.LinesDeleted))
	}

	sb.WriteString("\nRead these files and identify bugs, security issues, performance problems, and improvements.\n\n")
	sb.WriteString("Format each finding as:\n")
	sb.WriteString("FINDING: [file:line] [severity] [category]\n")
	sb.WriteString("DESCRIPTION: <description>\n")
	sb.WriteString("FIX: <suggested fix>\n")

	return sb.String()
}

// parseFindings parses findings from the AI response.
func parseFindings(response string) []session.Finding {
	var findings []session.Finding

	lines := strings.Split(response, "\n")
	var current *session.Finding

	for i := range lines {
		line := strings.TrimSpace(lines[i])

		if strings.HasPrefix(line, "FINDING:") {
			// Start a new finding
			if current != nil {
				findings = append(findings, *current)
			}

			current = parseFindingLine(line)
		} else if current != nil && strings.HasPrefix(line, "DESCRIPTION:") {
			desc := strings.TrimPrefix(line, "DESCRIPTION:")
			current.Description = strings.TrimSpace(desc)
		} else if current != nil && strings.HasPrefix(line, "FIX:") {
			fix := strings.TrimPrefix(line, "FIX:")
			current.SuggestedFix = strings.TrimSpace(fix)
		}
	}

	// Don't forget the last finding
	if current != nil {
		findings = append(findings, *current)
	}

	return findings
}

// parseFindingLine parses a FINDING: line.
// Format: FINDING: [file:line] [severity] [category].
func parseFindingLine(line string) *session.Finding {
	line = strings.TrimPrefix(line, "FINDING:")
	line = strings.TrimSpace(line)

	finding := &session.Finding{
		Severity: "warning",
		Category: "style",
	}

	// Parse [file:line]
	if idx := strings.Index(line, "]"); idx > 0 && strings.HasPrefix(line, "[") {
		location := line[1:idx]

		if colonIdx := strings.LastIndex(location, ":"); colonIdx > 0 {
			finding.File = location[:colonIdx]

			if lineNum := parseInt(location[colonIdx+1:]); lineNum > 0 {
				finding.Line = lineNum
			}
		} else {
			finding.File = location
		}

		line = strings.TrimSpace(line[idx+1:])
	}

	// Parse [severity]
	severities := []string{"error", "warning", "suggestion"}
	for _, sev := range severities {
		if strings.Contains(strings.ToLower(line), sev) {
			finding.Severity = sev

			break
		}
	}

	// Parse [category]
	categories := []string{"bug", "security", "performance", "style", "testing"}
	for _, cat := range categories {
		if strings.Contains(strings.ToLower(line), cat) {
			finding.Category = cat

			break
		}
	}

	return finding
}

// parseInt parses a string to int, returning 0 on error.
func parseInt(s string) int {
	var result int

	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		} else {
			break
		}
	}

	return result
}
