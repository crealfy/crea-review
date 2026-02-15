// Package priority provides file priority scoring for code reviews.
package priority

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crealfy/crea-pipe/pkg/git"
	rcontext "github.com/crealfy/crea-review/pkg/context"
)

// Score represents a file's priority score.
type Score struct {
	// Path is the file path.
	Path string

	// Total is the overall priority score (0-100).
	Total float64

	// LinesChanged is the number of lines changed.
	LinesChanged int

	// IsCriticalPath indicates if the file is in a critical path.
	IsCriticalPath bool

	// ChurnCount is the historical change frequency.
	ChurnCount int

	// HasTests indicates if the file has associated tests.
	HasTests bool

	// Breakdown contains the score components.
	Breakdown Breakdown
}

// Breakdown contains the individual score components.
type Breakdown struct {
	// LinesChangedScore is the score from lines changed (0-30).
	LinesChangedScore float64

	// CriticalityScore is the score from critical path detection (0-25).
	CriticalityScore float64

	// ChurnScore is the score from historical churn (0-20).
	ChurnScore float64

	// TestCoverageScore is the score from test coverage (0-15).
	TestCoverageScore float64

	// RecencyScore is the score from recency (0-10).
	RecencyScore float64
}

// Weights defines the scoring weights.
type Weights struct {
	LinesChanged float64
	Criticality  float64
	Churn        float64
	TestCoverage float64
	Recency      float64
}

// DefaultWeights returns the default scoring weights.
func DefaultWeights() Weights {
	return Weights{
		LinesChanged: 0.30,
		Criticality:  0.25,
		Churn:        0.20,
		TestCoverage: 0.15,
		Recency:      0.10,
	}
}

// Scorer calculates priority scores for files.
type Scorer struct {
	repoPath string
	weights  Weights
}

// NewScorer creates a new priority scorer.
func NewScorer(repoPath string) *Scorer {
	return &Scorer{
		repoPath: repoPath,
		weights:  DefaultWeights(),
	}
}

// WithWeights sets custom weights.
func (s *Scorer) WithWeights(w Weights) *Scorer {
	s.weights = w

	return s
}

// ScoreFiles scores a list of changed files by priority.
func (s *Scorer) ScoreFiles(ctx context.Context, files []rcontext.FileContent) ([]Score, error) {
	// Find max lines changed for normalization
	maxLines := 1

	for _, f := range files {
		total := f.LinesAdded + f.LinesDeleted
		if total > maxLines {
			maxLines = total
		}
	}

	// Build test file map for test detection
	testFiles := make(map[string]bool)

	for _, f := range files {
		if isTestFile(f.Path) {
			testFiles[f.Path] = true
		}
	}

	scores := make([]Score, 0, len(files))

	for _, f := range files {
		score := s.scoreFile(ctx, f, maxLines, testFiles)
		scores = append(scores, score)
	}

	// Sort by total score descending
	sortByScore(scores)

	return scores, nil
}

// scoreFile calculates the priority score for a single file.
func (s *Scorer) scoreFile(ctx context.Context, f rcontext.FileContent, maxLines int, testFiles map[string]bool) Score {
	linesChanged := f.LinesAdded + f.LinesDeleted

	// Lines changed score (0-30)
	linesScore := (float64(linesChanged) / float64(maxLines)) * 100 * s.weights.LinesChanged

	// Criticality score (0-25)
	isCritical := isCriticalPath(f.Path)
	criticalScore := 0.0
	if isCritical {
		criticalScore = 100 * s.weights.Criticality
	}

	// Churn score (0-20)
	churnCount := 0
	churnScore := 0.0

	churn, err := git.FileChurn(ctx, s.repoPath, f.Path)
	if err == nil && churn > 0 {
		churnCount = churn
		// Normalize: more churn = higher priority (capped at 50 commits)
		normalized := float64(churn)
		if normalized > 50 {
			normalized = 50
		}

		churnScore = (normalized / 50) * 100 * s.weights.Churn
	}

	// Test coverage score (0-15)
	// Files without tests get higher priority (need more scrutiny)
	hasTests := hasAssociatedTests(f.Path, testFiles)
	testScore := 0.0
	if !hasTests && !isTestFile(f.Path) {
		testScore = 100 * s.weights.TestCoverage
	}

	// Recency score (0-10)
	// For now, use a simple heuristic - files at the beginning of diff are newer
	// This could be improved with actual timestamp checking
	recencyScore := 50 * s.weights.Recency // Base score

	total := linesScore + criticalScore + churnScore + testScore + recencyScore

	return Score{
		Path:           f.Path,
		Total:          total,
		LinesChanged:   linesChanged,
		IsCriticalPath: isCritical,
		ChurnCount:     churnCount,
		HasTests:       hasTests,
		Breakdown: Breakdown{
			LinesChangedScore: linesScore,
			CriticalityScore:  criticalScore,
			ChurnScore:        churnScore,
			TestCoverageScore: testScore,
			RecencyScore:      recencyScore,
		},
	}
}

// Critical path patterns.
var criticalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)/auth/`),
	regexp.MustCompile(`(?i)/security/`),
	regexp.MustCompile(`(?i)/crypto/`),
	regexp.MustCompile(`(?i)/api/`),
	regexp.MustCompile(`(?i)/handler/`),
	regexp.MustCompile(`(?i)/controller/`),
	regexp.MustCompile(`(?i)/middleware/`),
	regexp.MustCompile(`(?i)/payment/`),
	regexp.MustCompile(`(?i)/billing/`),
	regexp.MustCompile(`(?i)/admin/`),
	regexp.MustCompile(`(?i)password`),
	regexp.MustCompile(`(?i)secret`),
	regexp.MustCompile(`(?i)token`),
	regexp.MustCompile(`(?i)credential`),
}

// isCriticalPath checks if a file path is in a critical area.
func isCriticalPath(path string) bool {
	for _, pattern := range criticalPatterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// isTestFile checks if a file is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)

	// Go tests
	if strings.HasSuffix(base, "_test.go") {
		return true
	}

	// JavaScript/TypeScript tests
	if strings.HasSuffix(base, ".test.js") ||
		strings.HasSuffix(base, ".test.ts") ||
		strings.HasSuffix(base, ".test.jsx") ||
		strings.HasSuffix(base, ".test.tsx") ||
		strings.HasSuffix(base, ".spec.js") ||
		strings.HasSuffix(base, ".spec.ts") {
		return true
	}

	// Python tests
	if strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") {
		return true
	}

	// Java/Kotlin tests
	if strings.HasSuffix(base, "Test.java") ||
		strings.HasSuffix(base, "Test.kt") {
		return true
	}

	// Check for test directories
	dir := filepath.Dir(path)

	return strings.Contains(dir, "/test/") ||
		strings.Contains(dir, "/tests/") ||
		strings.Contains(dir, "/__tests__/") ||
		strings.HasSuffix(dir, "/test") ||
		strings.HasSuffix(dir, "/tests")
}

// hasAssociatedTests checks if a source file has associated test files.
func hasAssociatedTests(path string, testFiles map[string]bool) bool {
	if isTestFile(path) {
		return true
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	dir := filepath.Dir(path)

	// Check common test file patterns
	testPatterns := []string{
		// Go
		filepath.Join(dir, base+"_test.go"),
		// JavaScript/TypeScript
		filepath.Join(dir, base+".test"+ext),
		filepath.Join(dir, base+".spec"+ext),
		filepath.Join(dir, "__tests__", base+ext),
		// Python
		filepath.Join(dir, "test_"+base+ext),
		filepath.Join(dir, base+"_test"+ext),
	}

	for _, pattern := range testPatterns {
		if testFiles[pattern] {
			return true
		}
	}

	return false
}

// sortByScore sorts scores by total descending.
func sortByScore(scores []Score) {
	for i := range scores {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].Total > scores[i].Total {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}
}
