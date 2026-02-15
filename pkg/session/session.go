// Package session provides review session management for incremental code reviews.
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// Status represents the status of a review session.
type Status string

const (
	StatusPending    Status = "pending"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
)

// Session represents a review session.
type Session struct {
	// ID is the session identifier.
	ID int `json:"id"`

	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`

	// BaseCommit is the base commit for comparison.
	BaseCommit string `json:"base_commit"`

	// HeadCommit is the head commit.
	HeadCommit string `json:"head_commit"`

	// TotalFilesInDiff is the total number of files in the diff.
	TotalFilesInDiff int `json:"total_files_in_diff"`

	// FilesReviewed is the number of files reviewed in this session.
	FilesReviewed int `json:"files_reviewed"`

	// FilesRemaining is the number of files remaining to review.
	FilesRemaining int `json:"files_remaining"`

	// Status is the session status.
	Status Status `json:"status"`

	// ContinuedFrom is the session ID this continues from (0 if first).
	ContinuedFrom int `json:"continued_from,omitempty"`

	// Files contains the files in this batch.
	Files []string `json:"files"`

	// Findings contains the review findings.
	Findings []Finding `json:"findings,omitempty"`
}

// Finding represents a review finding.
type Finding struct {
	// File is the file path.
	File string `json:"file"`

	// Line is the line number.
	Line int `json:"line"`

	// Severity is the severity level (error, warning, suggestion).
	Severity string `json:"severity"`

	// Category is the finding category (bug, security, performance, style).
	Category string `json:"category"`

	// Description is the finding description.
	Description string `json:"description"`

	// SuggestedFix is the suggested fix.
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

// Store manages review sessions for a project.
type Store struct {
	// ProjectPath is the repository root path.
	ProjectPath string

	// StateDir is the state directory path.
	StateDir string
}

// NewStore creates a new session store.
func NewStore(projectPath string, stateDir string) (*Store, error) {
	if stateDir == "" {
		// Default state directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}

		projectHash := hashPath(projectPath)
		stateDir = filepath.Join(home, ".valksor", "crealfy", "review", projectHash)
	}

	// Ensure state directory exists
	sessionsDir := filepath.Join(stateDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}

	// Write project metadata
	projectMeta := struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}{
		Path: projectPath,
		Name: filepath.Base(projectPath),
	}

	metaPath := filepath.Join(stateDir, "project.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		data, err := json.MarshalIndent(projectMeta, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("marshal project meta: %w", err)
		}

		if err := os.WriteFile(metaPath, data, 0o644); err != nil {
			return nil, fmt.Errorf("write project meta: %w", err)
		}
	}

	return &Store{
		ProjectPath: projectPath,
		StateDir:    stateDir,
	}, nil
}

// Create creates a new session.
func (s *Store) Create(session *Session) error {
	// Get next session ID
	sessions, err := s.List()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	maxID := 0
	for _, sess := range sessions {
		if sess.ID > maxID {
			maxID = sess.ID
		}
	}

	session.ID = maxID + 1
	session.CreatedAt = time.Now()

	if session.Status == "" {
		session.Status = StatusPending
	}

	return s.Save(session)
}

// Save saves a session to disk.
func (s *Store) Save(session *Session) error {
	sessionDir := filepath.Join(s.StateDir, "sessions", strconv.Itoa(session.ID))
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// Save metadata
	metaPath := filepath.Join(sessionDir, "meta.json")
	metaData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := atomicWrite(metaPath, metaData); err != nil {
		return fmt.Errorf("write session meta: %w", err)
	}

	// Update latest symlink
	latestPath := filepath.Join(s.StateDir, "sessions", "latest")
	_ = os.Remove(latestPath) // Ignore error if doesn't exist

	targetPath := strconv.Itoa(session.ID)
	if err := os.Symlink(targetPath, latestPath); err != nil {
		// Non-fatal, just log
		fmt.Fprintf(os.Stderr, "warning: failed to update latest symlink: %v\n", err)
	}

	return nil
}

// Load loads a session by ID.
func (s *Store) Load(id int) (*Session, error) {
	metaPath := filepath.Join(s.StateDir, "sessions", strconv.Itoa(id), "meta.json")

	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, fmt.Errorf("read session: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	return &session, nil
}

// LoadLatest loads the most recent session.
func (s *Store) LoadLatest() (*Session, error) {
	sessions, err := s.List()
	if err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, errors.New("no sessions found")
	}

	// Return the one with highest ID
	latest := sessions[0]
	for _, sess := range sessions[1:] {
		if sess.ID > latest.ID {
			latest = sess
		}
	}

	return latest, nil
}

// CollectReviewedFiles returns all files reviewed in a session chain,
// along with the root session that contains the original commits.
// It traverses from the given session back through ContinuedFrom links.
func (s *Store) CollectReviewedFiles(sessionID int) ([]string, *Session, error) {
	seen := make(map[string]bool)
	var rootSession *Session

	currentID := sessionID
	for currentID > 0 {
		sess, err := s.Load(currentID)
		if err != nil {
			return nil, nil, fmt.Errorf("load session %d: %w", currentID, err)
		}

		// Add files from this session
		for _, f := range sess.Files {
			seen[f] = true
		}

		// Track root session (where chain started)
		if sess.ContinuedFrom == 0 {
			rootSession = sess
		}

		currentID = sess.ContinuedFrom
	}

	// Convert map to slice
	files := make([]string, 0, len(seen))
	for f := range seen {
		files = append(files, f)
	}

	return files, rootSession, nil
}

// List returns all sessions sorted by ID.
func (s *Store) List() ([]*Session, error) {
	sessionsDir := filepath.Join(s.StateDir, "sessions")

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []*Session

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if entry.Name() == "latest" {
			continue
		}

		metaPath := filepath.Join(sessionsDir, entry.Name(), "meta.json")

		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue // Skip invalid sessions
		}

		var session Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}

		sessions = append(sessions, &session)
	}

	// Sort by ID
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ID < sessions[j].ID
	})

	return sessions, nil
}

// Delete deletes a session by ID.
func (s *Store) Delete(id int) error {
	sessionDir := filepath.Join(s.StateDir, "sessions", strconv.Itoa(id))

	return os.RemoveAll(sessionDir)
}

// hashPath creates a short hash of a path for directory naming.
func hashPath(path string) string {
	h := sha256.Sum256([]byte(path))

	return hex.EncodeToString(h[:8]) // First 16 hex chars
}

// atomicWrite writes data to a file atomically using temp file + rename.
func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}

	tmpPath := tmpFile.Name()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)

		return err
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)

		return err
	}

	return os.Rename(tmpPath, path)
}
