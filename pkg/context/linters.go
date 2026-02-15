// Package context provides review context gathering functionality.
package context

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// LinterOptions configures linter execution.
type LinterOptions struct {
	// Command is the linter command (e.g., "golangci-lint run --out-format json").
	Command string

	// RepoPath is the repository root directory.
	RepoPath string

	// Files are the files to lint (appended to command).
	Files []string

	// All runs the linter on the entire repo, ignoring Files.
	All bool
}

// RunLinter executes a user-provided linter command.
// Returns findings parsed from output, or error if linter fails.
func RunLinter(ctx context.Context, opts LinterOptions) ([]LinterFinding, error) {
	if opts.Command == "" {
		return nil, errors.New("linter command required")
	}

	// Build command with files appended (unless All is set)
	cmdStr := opts.Command
	if !opts.All && len(opts.Files) > 0 {
		cmdStr = cmdStr + " " + strings.Join(opts.Files, " ")
	}

	// Execute via shell for proper arg parsing
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	cmd.Dir = opts.RepoPath

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Parse output - try JSON first, fallback to raw
	findings := parseOutput(stdout.Bytes(), stderr.Bytes())

	// Fail if linter returned error AND produced no findings
	// (some linters exit non-zero when findings exist)
	if err != nil && len(findings) == 0 {
		return nil, fmt.Errorf("linter failed: %w\nstderr: %s", err, stderr.String())
	}

	return findings, nil
}

// parseOutput attempts to parse linter output as JSON, falls back to raw.
func parseOutput(stdout, stderr []byte) []LinterFinding {
	// Try JSON array of objects with file/line/message fields
	var findings []LinterFinding
	if json.Unmarshal(stdout, &findings) == nil {
		// Valid JSON array - return findings (may be empty/nil)
		if len(findings) == 0 {
			return nil
		}

		return findings
	}

	// Try JSON with nested Issues structure (common in golangci-lint)
	var nested struct {
		Issues []LinterFinding `json:"Issues"`
	}

	if json.Unmarshal(stdout, &nested) == nil {
		// Valid nested JSON - return issues (may be empty/nil)
		if len(nested.Issues) == 0 {
			return nil
		}

		return nested.Issues
	}

	// Fallback: wrap raw output as single finding
	combined := string(stdout)
	if len(stderr) > 0 {
		combined += "\n" + string(stderr)
	}

	if strings.TrimSpace(combined) == "" {
		return nil
	}

	return []LinterFinding{{
		Tool:    "linter",
		Message: strings.TrimSpace(combined),
		Level:   "info",
	}}
}
