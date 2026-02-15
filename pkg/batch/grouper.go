// Package batch provides smart batching of files for code reviews.
package batch

import (
	"path/filepath"
	"strings"

	"github.com/crealfy/crea-review/pkg/priority"
)

// Batch represents a batch of related files to review together.
type Batch struct {
	// ID is the batch identifier.
	ID int

	// Files contains the file paths in this batch.
	Files []string

	// Reason explains why these files are grouped.
	Reason string

	// TotalScore is the sum of priority scores.
	TotalScore float64
}

// Options configures the batching behavior.
type Options struct {
	// MaxFilesPerBatch is the maximum files per batch.
	MaxFilesPerBatch int

	// MaxTotalFiles is the maximum total files to include.
	MaxTotalFiles int

	// GroupByPackage groups files by package/directory.
	GroupByPackage bool

	// PairTests keeps source and test files together.
	PairTests bool
}

// DefaultOptions returns default batching options.
func DefaultOptions() Options {
	return Options{
		MaxFilesPerBatch: 50,
		MaxTotalFiles:    0, // Unlimited
		GroupByPackage:   true,
		PairTests:        true,
	}
}

// Grouper groups files into batches.
type Grouper struct {
	opts Options
}

// NewGrouper creates a new file grouper.
func NewGrouper(opts Options) *Grouper {
	return &Grouper{opts: opts}
}

// Group organizes scored files into batches.
func (g *Grouper) Group(scores []priority.Score) []Batch {
	if len(scores) == 0 {
		return nil
	}

	// Apply max files limit
	files := scores
	if g.opts.MaxTotalFiles > 0 && len(files) > g.opts.MaxTotalFiles {
		files = files[:g.opts.MaxTotalFiles]
	}

	// First, pair tests with their source files
	pairs := make(map[string][]priority.Score)
	used := make(map[string]bool)

	if g.opts.PairTests {
		for _, score := range files {
			if isTestFile(score.Path) {
				sourcePath := getSourcePath(score.Path)
				pairs[sourcePath] = append(pairs[sourcePath], score)
				used[score.Path] = true
			}
		}

		// Add source files to their pairs
		for _, score := range files {
			if !isTestFile(score.Path) {
				if _, hasPair := pairs[score.Path]; hasPair {
					pairs[score.Path] = append([]priority.Score{score}, pairs[score.Path]...)
					used[score.Path] = true
				}
			}
		}
	}

	// Group remaining files by package/directory
	packages := make(map[string][]priority.Score)

	if g.opts.GroupByPackage {
		for _, score := range files {
			if used[score.Path] {
				continue
			}

			pkg := filepath.Dir(score.Path)
			packages[pkg] = append(packages[pkg], score)
			used[score.Path] = true
		}
	}

	// Collect ungrouped files
	var ungrouped []priority.Score

	for _, score := range files {
		if !used[score.Path] {
			ungrouped = append(ungrouped, score)
		}
	}

	// Build batches
	var batches []Batch
	batchID := 1

	// First, add test pairs
	for sourcePath, pairFiles := range pairs {
		if len(pairFiles) == 0 {
			continue
		}

		batch := Batch{
			ID:     batchID,
			Reason: "test-pair",
		}

		for _, f := range pairFiles {
			batch.Files = append(batch.Files, f.Path)
			batch.TotalScore += f.Total
		}

		// Check if we need to split this pair
		if len(batch.Files) <= g.opts.MaxFilesPerBatch {
			batches = append(batches, batch)
			batchID++
		} else {
			// Split large pairs
			for i := 0; i < len(pairFiles); i += g.opts.MaxFilesPerBatch {
				end := i + g.opts.MaxFilesPerBatch
				if end > len(pairFiles) {
					end = len(pairFiles)
				}

				splitBatch := Batch{
					ID:     batchID,
					Reason: "test-pair (split " + filepath.Base(sourcePath) + ")",
				}

				for _, f := range pairFiles[i:end] {
					splitBatch.Files = append(splitBatch.Files, f.Path)
					splitBatch.TotalScore += f.Total
				}

				batches = append(batches, splitBatch)
				batchID++
			}
		}
	}

	// Then, add package groups
	for pkg, pkgFiles := range packages {
		// Split large packages into multiple batches
		for i := 0; i < len(pkgFiles); i += g.opts.MaxFilesPerBatch {
			end := i + g.opts.MaxFilesPerBatch
			if end > len(pkgFiles) {
				end = len(pkgFiles)
			}

			batch := Batch{
				ID:     batchID,
				Reason: "same-package (" + pkg + ")",
			}

			for _, f := range pkgFiles[i:end] {
				batch.Files = append(batch.Files, f.Path)
				batch.TotalScore += f.Total
			}

			batches = append(batches, batch)
			batchID++
		}
	}

	// Finally, add ungrouped files
	for i := 0; i < len(ungrouped); i += g.opts.MaxFilesPerBatch {
		end := i + g.opts.MaxFilesPerBatch
		if end > len(ungrouped) {
			end = len(ungrouped)
		}

		batch := Batch{
			ID:     batchID,
			Reason: "ungrouped",
		}

		for _, f := range ungrouped[i:end] {
			batch.Files = append(batch.Files, f.Path)
			batch.TotalScore += f.Total
		}

		batches = append(batches, batch)
		batchID++
	}

	// Sort batches by total score descending
	sortBatches(batches)

	return batches
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

	return false
}

// getSourcePath returns the source file path for a test file.
func getSourcePath(testPath string) string {
	dir := filepath.Dir(testPath)
	base := filepath.Base(testPath)
	ext := filepath.Ext(base)

	// Go: handler_test.go -> handler.go
	if strings.HasSuffix(base, "_test.go") {
		return filepath.Join(dir, strings.TrimSuffix(base, "_test.go")+".go")
	}

	// JavaScript/TypeScript: auth.test.js -> auth.js
	if strings.Contains(base, ".test.") {
		return filepath.Join(dir, strings.Replace(base, ".test.", ".", 1))
	}
	if strings.Contains(base, ".spec.") {
		return filepath.Join(dir, strings.Replace(base, ".spec.", ".", 1))
	}

	// Python: test_auth.py -> auth.py or auth_test.py -> auth.py
	if strings.HasPrefix(base, "test_") {
		return filepath.Join(dir, strings.TrimPrefix(base, "test_"))
	}
	if strings.HasSuffix(base, "_test.py") {
		return filepath.Join(dir, strings.TrimSuffix(base, "_test.py")+".py")
	}

	// Java/Kotlin: AuthTest.java -> Auth.java
	if strings.HasSuffix(base, "Test.java") {
		return filepath.Join(dir, strings.TrimSuffix(base, "Test.java")+".java")
	}
	if strings.HasSuffix(base, "Test.kt") {
		return filepath.Join(dir, strings.TrimSuffix(base, "Test.kt")+".kt")
	}

	// Fallback: remove test from name
	return filepath.Join(dir, strings.TrimSuffix(base, ext)+ext)
}

// sortBatches sorts batches by total score descending.
func sortBatches(batches []Batch) {
	for i := range batches {
		for j := i + 1; j < len(batches); j++ {
			if batches[j].TotalScore > batches[i].TotalScore {
				batches[i], batches[j] = batches[j], batches[i]
			}
		}
	}
}

// FilesInBatches returns the total number of files across all batches.
func FilesInBatches(batches []Batch) int {
	total := 0
	for _, b := range batches {
		total += len(b.Files)
	}

	return total
}

// TakeBatches returns the first n batches.
func TakeBatches(batches []Batch, n int) []Batch {
	if n >= len(batches) {
		return batches
	}

	return batches[:n]
}

// TakeFiles returns batches containing at most n files total.
func TakeFiles(batches []Batch, maxFiles int) []Batch {
	var result []Batch
	fileCount := 0

	for _, b := range batches {
		if fileCount+len(b.Files) > maxFiles {
			break
		}

		result = append(result, b)
		fileCount += len(b.Files)
	}

	return result
}
