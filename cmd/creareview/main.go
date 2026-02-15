// Package main provides the creareview CLI entry point.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/crealfy/crea-pipe/pkg/agent"
	"github.com/crealfy/crea-pipe/pkg/git"
	rcontext "github.com/crealfy/crea-review/pkg/context"
	"github.com/crealfy/crea-review/pkg/output"
	"github.com/crealfy/crea-review/pkg/priority"
	"github.com/crealfy/crea-review/pkg/review"
	"github.com/crealfy/crea-review/pkg/session"
)

// envVars is a flag.Value that collects KEY=VALUE pairs.
type envVars map[string]string

func (e *envVars) String() string {
	if e == nil || *e == nil {
		return ""
	}

	var pairs []string
	for k, v := range *e {
		pairs = append(pairs, k+"="+v)
	}

	return strings.Join(pairs, ",")
}

func (e *envVars) Set(value string) error {
	if *e == nil {
		*e = make(map[string]string)
	}

	k, v, ok := strings.Cut(value, "=")
	if !ok {
		return fmt.Errorf("invalid env format %q, expected KEY=VALUE", value)
	}

	(*e)[k] = v

	return nil
}

// CLI flags.
var (
	// CodeRabbit-compatible flags.
	reviewType = flag.String("t", "all", "Review type: all, committed, uncommitted")
	baseBranch = flag.String("base", "", "Base branch for comparison")
	baseCommit = flag.String("base-commit", "", "Base commit for comparison")
	cwd        = flag.String("cwd", "", "Working directory")
	plain      = flag.Bool("plain", false, "Output plain text format")
	promptOnly = flag.Bool("prompt-only", false, "Output minimal prompt for piping")
	noColor    = flag.Bool("no-color", false, "Disable colored output")

	// creareview specific flags.
	backend      = flag.String("backend", "claude", "AI backend: claude, codex")
	withLinters  = flag.Bool("with-linters", false, "Include linter output")
	linterCmd    = flag.String("linter", "", "Linter command to run (requires --with-linters)")
	lintAll      = flag.Bool("lint-all", false, "Lint entire repo instead of just changed files")
	quiet        = flag.Bool("quiet", false, "Suppress progress messages")

	// File limit and sorting.
	maxFiles = flag.Int("max-files", 15, "Max files per review batch")
	onLimit  = flag.String("on-limit", "continue", "When over max-files: continue, stop")
	sortBy   = flag.String("sort", "priority", "Sort: priority, alpha, none")

	// Session flags.
	continueFrom = flag.Int("continue", 0, "Continue from session N")
	listSessions = flag.Bool("list-sessions", false, "List all sessions")
	stateDir     = flag.String("state-dir", "", "Override state directory")

	// Model override.
	model = flag.String("model", "", "Model override")

	// Retry configuration.
	retries      = flag.Int("retries", 0, "Number of retries on transient failures")
	retryDelayMS = flag.Int("retry-delay", 1000, "Delay between retries in ms")

	// Environment variables.
	env envVars
)

func init() {
	flag.Var(&env, "env", "Environment variable KEY=VALUE (repeatable)")
}

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)

		return 1
	}

	return 0
}

func run(ctx context.Context) error {
	flag.Usage = usage
	flag.Parse()

	// Validate linter flags
	if *withLinters && *linterCmd == "" {
		return errors.New("--with-linters requires --linter to specify the linter command")
	}

	// Resolve working directory
	workDir := *cwd
	if workDir == "" {
		var err error

		workDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	// Get repo root
	repoRoot, err := git.RepoRoot(ctx, workDir)
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Initialize session store
	store, err := session.NewStore(repoRoot, *stateDir)
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}

	// Handle list-sessions
	if *listSessions {
		sessions, err := store.List()
		if err != nil {
			return fmt.Errorf("list sessions: %w", err)
		}

		return output.FormatSessionList(os.Stdout, sessions)
	}

	// Handle --continue flag
	var excludeFiles []string
	if *continueFrom > 0 {
		reviewed, rootSession, err := store.CollectReviewedFiles(*continueFrom)
		if err != nil {
			return fmt.Errorf("load session %d: %w", *continueFrom, err)
		}

		excludeFiles = reviewed

		// Use original session's commits if not overridden
		if *baseCommit == "" && *baseBranch == "" {
			*baseCommit = rootSession.BaseCommit
		}
	}

	// Determine output format
	format := output.FormatJSON
	if *plain {
		format = output.FormatPlain
	} else if *promptOnly {
		format = output.FormatPromptOnly
	}

	// Progress output
	progress := func(msg string) {
		if !*quiet && !*promptOnly {
			fmt.Fprintf(os.Stderr, "%s\n", msg)
		}
	}

	// Gather context
	progress("[1/4] Gathering context...")

	gatherOpts := rcontext.GatherOptions{
		BaseCommit:     *baseCommit,
		BaseBranch:     *baseBranch,
		ReviewType:     *reviewType,
		IncludeLinters: *withLinters,
		LinterCommand:  *linterCmd,
		LintAll:        *lintAll,
		MaxFiles:       0, // Don't limit here, we'll do it after scoring
		ExcludeFiles:   excludeFiles,
	}

	reviewCtx, err := rcontext.Gather(ctx, repoRoot, gatherOpts)
	if err != nil {
		return fmt.Errorf("gather context: %w", err)
	}

	if len(reviewCtx.ChangedFiles) == 0 {
		progress("No changes to review.")

		return nil
	}

	progress(fmt.Sprintf("   Found %d changed files", len(reviewCtx.ChangedFiles)))

	// Score files by priority
	progress("[2/4] Scoring files by priority...")

	scorer := priority.NewScorer(repoRoot)

	scores, err := scorer.ScoreFiles(ctx, reviewCtx.ChangedFiles)
	if err != nil {
		return fmt.Errorf("score files: %w", err)
	}

	// Apply sorting
	scores = sortScores(scores, *sortBy)

	// Apply max files limit
	filesToReview := scores
	if *maxFiles > 0 && len(filesToReview) > *maxFiles {
		if *onLimit == "stop" {
			return fmt.Errorf("too many files: %d (max %d). Use --on-limit continue or increase --max-files",
				len(filesToReview), *maxFiles)
		}

		filesToReview = filesToReview[:*maxFiles]
		progress(fmt.Sprintf("   Reviewing top %d files (by priority), %d remaining",
			*maxFiles, len(scores)-*maxFiles))
	}

	// Filter context to only include files we're reviewing
	reviewCtx.ChangedFiles = filterFiles(reviewCtx.ChangedFiles, filesToReview)
	reviewCtx.Stats.ReviewedFiles = len(reviewCtx.ChangedFiles)
	reviewCtx.Stats.SkippedFiles = len(scores) - len(reviewCtx.ChangedFiles)

	// Create session
	totalInDiff := len(scores)
	if *continueFrom > 0 {
		totalInDiff += len(excludeFiles) // Include previously reviewed files
	}

	sess := &session.Session{
		BaseCommit:       reviewCtx.BaseCommit,
		HeadCommit:       reviewCtx.HeadCommit,
		TotalFilesInDiff: totalInDiff,
		FilesReviewed:    len(reviewCtx.ChangedFiles),
		FilesRemaining:   len(scores) - len(reviewCtx.ChangedFiles),
		Status:           session.StatusInProgress,
		ContinuedFrom:    *continueFrom,
	}

	for _, f := range reviewCtx.ChangedFiles {
		sess.Files = append(sess.Files, f.Path)
	}

	if err := store.Create(sess); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	progress(fmt.Sprintf("   Session %d created", sess.ID))

	// Run review
	progress("[3/4] Running AI review...")

	reviewer, err := review.NewReviewer(review.Backend(*backend))
	if err != nil {
		return fmt.Errorf("init reviewer: %w", err)
	}

	reviewOpts := review.Options{
		Model:        *model,
		Env:          env,
		Retries:      *retries,
		RetryDelayMS: *retryDelayMS,
	}

	if !*quiet && !*promptOnly {
		reviewOpts.StreamHandler = func(event agent.Event) {
			// Could show progress dots or status here
		}
	}

	result, err := reviewer.Review(ctx, reviewCtx, reviewOpts)
	if err != nil {
		return fmt.Errorf("run review: %w", err)
	}

	// Update session with findings
	sess.Findings = result.Findings
	sess.Status = session.StatusCompleted

	if err := store.Save(sess); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	// Format output
	progress("[4/4] Formatting output...")

	formatter := output.NewFormatter(format)
	if *noColor {
		formatter = formatter.WithNoColor()
	}

	if err := formatter.Format(os.Stdout, result, sess); err != nil {
		return fmt.Errorf("format output: %w", err)
	}

	// Show continuation hint
	if sess.FilesRemaining > 0 && !*promptOnly {
		fmt.Fprintf(os.Stderr, "\nRun 'creareview --continue %d' for next batch (%d files remaining)\n",
			sess.ID, sess.FilesRemaining)
	}

	return nil
}

// sortScores sorts files based on the sort order.
func sortScores(scores []priority.Score, sortOrder string) []priority.Score {
	switch sortOrder {
	case "alpha":
		// Sort alphabetically by path
		for i := range scores {
			for j := i + 1; j < len(scores); j++ {
				if scores[j].Path < scores[i].Path {
					scores[i], scores[j] = scores[j], scores[i]
				}
			}
		}
	case "none":
		// Keep original order (no sorting needed, but scores come pre-sorted by priority)
		// Return as-is from gather order
	default: // "priority"
		// Already sorted by priority from ScoreFiles
	}

	return scores
}

// filterFiles returns only the files that match the scored files.
func filterFiles(files []rcontext.FileContent, scores []priority.Score) []rcontext.FileContent {
	scoreMap := make(map[string]bool)
	for _, s := range scores {
		scoreMap[s.Path] = true
	}

	var result []rcontext.FileContent

	for _, f := range files {
		if scoreMap[f.Path] {
			result = append(result, f)
		}
	}

	return result
}

func usage() {
	fmt.Fprintf(os.Stderr, `creareview - AI Code Review Tool

Usage: creareview [flags]

CodeRabbit-compatible flags:
  -t string           Review type: all, committed, uncommitted (default "all")
  --base string       Base branch for comparison
  --base-commit string Base commit for comparison
  --cwd string        Working directory
  --plain             Output plain text format
  --prompt-only       Output minimal prompt for piping to crea-pipe
  --no-color          Disable colored output

creareview specific flags:
  --backend string    AI backend: claude, codex (default "claude")
  -env KEY=VALUE      Environment variable (repeatable)
  --with-linters      Include linter output
  --linter string     Linter command to run (requires --with-linters)
  --lint-all          Lint entire repo instead of just changed files
  --quiet             Suppress progress messages
  --model string      Model override
  --retries int       Number of retries on transient failures (default 0)
  --retry-delay int   Delay between retries in ms (default 1000)

File limit and sorting:
  --max-files int     Max files per review batch (default 15)
  --on-limit string   When over max-files: continue, stop (default "continue")
  --sort string       Sort files: priority, alpha, none (default "priority")

Session management:
  --continue int      Continue from session N
  --list-sessions     List all sessions
  --state-dir string  Override state directory

Examples:
  # Review uncommitted changes
  creareview -t uncommitted --plain

  # Compare against main branch
  creareview --base main --prompt-only | crea-pipe --auto-approve

  # Include golangci-lint findings in review
  creareview --base main --with-linters --linter "golangci-lint run --out-format json"

  # Continue from previous session
  creareview --continue 1

`)
}
