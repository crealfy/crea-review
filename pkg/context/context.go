// Package context provides review context gathering functionality.
package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crealfy/crea-pipe/pkg/git"
)

// ReviewContext contains all the context needed for a code review.
type ReviewContext struct {
	// RepoPath is the repository root path.
	RepoPath string

	// BaseCommit is the base commit for comparison.
	BaseCommit string

	// HeadCommit is the head commit (usually HEAD).
	HeadCommit string

	// BaseBranch is the base branch name (if comparing branches).
	BaseBranch string

	// Diff is the raw unified diff.
	Diff string

	// ChangedFiles contains the changed files with their contents.
	ChangedFiles []FileContent

	// RelatedFiles contains files related to the changes.
	RelatedFiles []FileContent

	// LinterOutput contains optional linter findings.
	LinterOutput []LinterFinding

	// Stats contains review statistics.
	Stats ReviewStats
}

// FileContent represents a file with its content.
type FileContent struct {
	// Path is the file path relative to repo root.
	Path string

	// Language is the detected programming language.
	Language string

	// Content is the file content.
	Content string

	// Truncated indicates if the content was truncated.
	Truncated bool

	// LinesTotal is the total number of lines.
	LinesTotal int

	// LinesAdded is the number of lines added in the diff.
	LinesAdded int

	// LinesDeleted is the number of lines deleted in the diff.
	LinesDeleted int

	// Status is the file status (added, modified, deleted, renamed).
	Status string

	// OldPath is the previous path for renamed files.
	OldPath string

	// IsBinary indicates if this is a binary file.
	IsBinary bool

	// RelatedReason explains why this file is related (for RelatedFiles).
	RelatedReason string
}

// LinterFinding represents a linter finding.
type LinterFinding struct {
	// Tool is the linter name.
	Tool string `json:"Tool,omitempty"`

	// File is the file path.
	File string `json:"File,omitempty"`

	// Line is the line number.
	Line int `json:"Line,omitempty"`

	// Column is the column number.
	Column int `json:"Column,omitempty"`

	// Level is the severity (error, warning, info).
	Level string `json:"Level,omitempty"`

	// Message is the finding message.
	Message string `json:"Message,omitempty"`

	// RuleID is the rule identifier.
	RuleID string `json:"RuleID,omitempty"`
}

// ReviewStats contains statistics about the review context.
type ReviewStats struct {
	// TotalFiles is the total number of files in the diff.
	TotalFiles int

	// ReviewedFiles is the number of files included in this review.
	ReviewedFiles int

	// SkippedFiles is the number of files skipped.
	SkippedFiles int

	// TotalLinesAdded is the total lines added.
	TotalLinesAdded int

	// TotalLinesDeleted is the total lines deleted.
	TotalLinesDeleted int

	// BinaryFiles is the number of binary files skipped.
	BinaryFiles int
}

// GatherOptions configures context gathering.
type GatherOptions struct {
	// BaseCommit is the base commit for comparison.
	BaseCommit string

	// HeadCommit is the head commit (defaults to HEAD).
	HeadCommit string

	// BaseBranch is the base branch for comparison.
	BaseBranch string

	// ReviewType is the type of review (all, committed, uncommitted).
	ReviewType string

	// IncludeLinters runs linters and includes output.
	IncludeLinters bool

	// LinterCommand is the linter command to run (e.g., "golangci-lint run --out-format json").
	LinterCommand string

	// LintAll runs linter on entire repo, not just changed files.
	LintAll bool

	// MaxFiles limits the number of files to gather.
	MaxFiles int

	// ExcludeFiles is a list of file paths to exclude from gathering.
	// Used when continuing from a previous session to skip already-reviewed files.
	ExcludeFiles []string
}

// DefaultGatherOptions returns default gather options.
func DefaultGatherOptions() GatherOptions {
	return GatherOptions{
		HeadCommit: "HEAD",
		ReviewType: "all",
		MaxFiles:   50,
	}
}

// Gather collects all context needed for a code review.
func Gather(ctx context.Context, repoPath string, opts GatherOptions) (*ReviewContext, error) {
	// Resolve repo root
	root, err := git.RepoRoot(ctx, repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo root: %w", err)
	}

	rc := &ReviewContext{
		RepoPath: root,
	}

	// Determine base and head commits
	if err := resolveCommits(ctx, rc, opts); err != nil {
		return nil, fmt.Errorf("resolve commits: %w", err)
	}

	// Get raw diff
	diff, err := git.Diff(ctx, root, rc.BaseCommit, rc.HeadCommit)
	if err != nil {
		return nil, fmt.Errorf("get diff: %w", err)
	}
	rc.Diff = diff

	// Get structured file list
	diffFiles, err := git.DiffFiles(ctx, root, rc.BaseCommit, rc.HeadCommit)
	if err != nil {
		return nil, fmt.Errorf("get diff files: %w", err)
	}

	// Gather file contents
	rc.ChangedFiles, rc.Stats = gatherFileContents(ctx, root, diffFiles, opts)

	// Skip related files gathering - Claude reads files itself

	// Run linters if requested
	if opts.IncludeLinters {
		findings, err := runLinters(ctx, root, rc.ChangedFiles, opts)
		if err != nil {
			// Non-fatal
			fmt.Fprintf(os.Stderr, "warning: failed to run linters: %v\n", err)
		} else {
			rc.LinterOutput = findings
		}
	}

	return rc, nil
}

// resolveCommits determines the base and head commits based on options.
func resolveCommits(ctx context.Context, rc *ReviewContext, opts GatherOptions) error {
	// If base commit is specified, use it
	if opts.BaseCommit != "" {
		rc.BaseCommit = opts.BaseCommit
		rc.HeadCommit = opts.HeadCommit

		return nil
	}

	// If base branch is specified, find merge base
	if opts.BaseBranch != "" {
		rc.BaseBranch = opts.BaseBranch
		head, err := git.HEAD(ctx, rc.RepoPath)
		if err != nil {
			return fmt.Errorf("get HEAD: %w", err)
		}
		rc.HeadCommit = head

		// For branch comparison, we'll use the branch name directly
		// The Diff function handles this
		rc.BaseCommit = opts.BaseBranch

		return nil
	}

	// Handle review types
	switch opts.ReviewType {
	case "uncommitted":
		// Compare against HEAD for uncommitted changes
		rc.BaseCommit = "HEAD"
		rc.HeadCommit = "" // Empty means working directory
	case "committed":
		// Compare HEAD against its parent
		rc.BaseCommit = "HEAD~1"
		rc.HeadCommit = "HEAD"
	default: // "all"
		// Compare against HEAD (shows all uncommitted changes)
		rc.BaseCommit = "HEAD"
		rc.HeadCommit = ""
	}

	return nil
}

// gatherFileContents collects metadata about changed files (no content - Claude reads files itself).
func gatherFileContents(_ context.Context, _ string, diffFiles []git.DiffFile, opts GatherOptions) ([]FileContent, ReviewStats) {
	var files []FileContent
	stats := ReviewStats{
		TotalFiles: len(diffFiles),
	}

	// Build exclusion set for already-reviewed files
	excludeSet := make(map[string]bool)
	for _, f := range opts.ExcludeFiles {
		excludeSet[f] = true
	}

	for _, df := range diffFiles {
		// Skip excluded files (already reviewed in previous session)
		if excludeSet[df.Path] {
			stats.SkippedFiles++

			continue
		}

		// Skip binary files
		if df.IsBinary {
			stats.BinaryFiles++

			continue
		}

		// Check max files limit
		if opts.MaxFiles > 0 && len(files) >= opts.MaxFiles {
			stats.SkippedFiles++

			continue
		}

		stats.TotalLinesAdded += df.LinesAdded
		stats.TotalLinesDeleted += df.LinesDeleted

		// Only collect metadata - Claude reads files itself
		fc := FileContent{
			Path:         df.Path,
			OldPath:      df.OldPath,
			Status:       string(df.Status),
			LinesAdded:   df.LinesAdded,
			LinesDeleted: df.LinesDeleted,
			Language:     detectLanguage(df.Path),
		}

		files = append(files, fc)
		stats.ReviewedFiles++
	}

	return files, stats
}


// detectLanguage detects the programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".jsx":
		return "jsx"
	case ".tsx":
		return "tsx"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".h", ".hpp":
		return "c-header"
	case ".cs":
		return "csharp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".sh", ".bash":
		return "shell"
	case ".sql":
		return "sql"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".xml":
		return "xml"
	case ".md", ".markdown":
		return "markdown"
	case ".proto":
		return "protobuf"
	default:
		return "text"
	}
}


// runLinters runs the configured linter on changed files.
func runLinters(ctx context.Context, repoPath string, files []FileContent, opts GatherOptions) ([]LinterFinding, error) {
	if opts.LinterCommand == "" {
		return nil, nil // No linter configured
	}

	// Extract file paths (skip deleted files)
	var filePaths []string

	for _, f := range files {
		if f.Status != "deleted" {
			filePaths = append(filePaths, f.Path)
		}
	}

	return RunLinter(ctx, LinterOptions{
		Command:  opts.LinterCommand,
		RepoPath: repoPath,
		Files:    filePaths,
		All:      opts.LintAll,
	})
}
