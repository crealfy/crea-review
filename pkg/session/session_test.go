package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := "/test/project"

	store, err := NewStore(projectPath, tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if store.ProjectPath != projectPath {
		t.Errorf("ProjectPath = %q, want %q", store.ProjectPath, projectPath)
	}

	if store.StateDir != tmpDir {
		t.Errorf("StateDir = %q, want %q", store.StateDir, tmpDir)
	}

	// Check sessions directory was created
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		t.Error("sessions directory was not created")
	}

	// Check project.json was created
	projectMeta := filepath.Join(tmpDir, "project.json")
	if _, err := os.Stat(projectMeta); os.IsNotExist(err) {
		t.Error("project.json was not created")
	}
}

func TestStoreCreateAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	session := &Session{
		BaseCommit:       "abc123",
		HeadCommit:       "def456",
		TotalFilesInDiff: 100,
		FilesReviewed:    50,
		FilesRemaining:   50,
		Status:           StatusInProgress,
		Files:            []string{"main.go", "handler.go"},
	}

	if err := store.Create(session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.ID != 1 {
		t.Errorf("ID = %d, want 1", session.ID)
	}

	if session.CreatedAt.IsZero() {
		t.Error("CreatedAt was not set")
	}

	// Load the session
	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.BaseCommit != session.BaseCommit {
		t.Errorf("BaseCommit = %q, want %q", loaded.BaseCommit, session.BaseCommit)
	}

	if loaded.FilesReviewed != session.FilesReviewed {
		t.Errorf("FilesReviewed = %d, want %d", loaded.FilesReviewed, session.FilesReviewed)
	}

	if len(loaded.Files) != 2 {
		t.Errorf("len(Files) = %d, want 2", len(loaded.Files))
	}
}

func TestStoreList(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create multiple sessions
	for range 3 {
		session := &Session{
			BaseCommit:       "abc123",
			TotalFilesInDiff: 100,
			FilesReviewed:    50,
			Status:           StatusCompleted,
		}
		if err := store.Create(session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	sessions, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 3 {
		t.Errorf("len(sessions) = %d, want 3", len(sessions))
	}

	// Verify sorted by ID
	for i := 1; i < len(sessions); i++ {
		if sessions[i].ID <= sessions[i-1].ID {
			t.Error("sessions not sorted by ID")
		}
	}
}

func TestStoreLoadLatest(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create multiple sessions
	for i := range 3 {
		session := &Session{
			BaseCommit: "commit" + string(rune('a'+i)),
			Status:     StatusCompleted,
		}
		if err := store.Create(session); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	latest, err := store.LoadLatest()
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}

	if latest.ID != 3 {
		t.Errorf("latest.ID = %d, want 3", latest.ID)
	}
}

func TestStoreLoadLatestEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, err = store.LoadLatest()
	if err == nil {
		t.Error("LoadLatest() should error when no sessions exist")
	}
}

func TestStoreDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	session := &Session{
		BaseCommit: "abc123",
		Status:     StatusCompleted,
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := store.Delete(1); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.Load(1)
	if err == nil {
		t.Error("Load() should error after Delete()")
	}
}

func TestStoreContinuedSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create first session
	session1 := &Session{
		BaseCommit:       "abc123",
		TotalFilesInDiff: 100,
		FilesReviewed:    50,
		FilesRemaining:   50,
		Status:           StatusCompleted,
	}
	if err := store.Create(session1); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create continued session
	session2 := &Session{
		BaseCommit:       "abc123",
		TotalFilesInDiff: 100,
		FilesReviewed:    50,
		FilesRemaining:   0,
		Status:           StatusCompleted,
		ContinuedFrom:    1,
	}
	if err := store.Create(session2); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := store.Load(2)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if loaded.ContinuedFrom != 1 {
		t.Errorf("ContinuedFrom = %d, want 1", loaded.ContinuedFrom)
	}
}

func TestCollectReviewedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create session chain: 1 -> 2 -> 3
	session1 := &Session{
		BaseCommit: "abc123",
		HeadCommit: "def456",
		Status:     StatusCompleted,
		Files:      []string{"a.go", "b.go"},
	}
	if err := store.Create(session1); err != nil {
		t.Fatalf("Create session 1 error = %v", err)
	}

	session2 := &Session{
		BaseCommit:    "abc123",
		HeadCommit:    "def456",
		Status:        StatusCompleted,
		ContinuedFrom: 1,
		Files:         []string{"c.go", "d.go"},
	}
	if err := store.Create(session2); err != nil {
		t.Fatalf("Create session 2 error = %v", err)
	}

	session3 := &Session{
		BaseCommit:    "abc123",
		HeadCommit:    "def456",
		Status:        StatusCompleted,
		ContinuedFrom: 2,
		Files:         []string{"e.go"},
	}
	if err := store.Create(session3); err != nil {
		t.Fatalf("Create session 3 error = %v", err)
	}

	// Test from session 3
	files, root, err := store.CollectReviewedFiles(3)
	if err != nil {
		t.Fatalf("CollectReviewedFiles(3) error = %v", err)
	}

	// Should have all 5 files
	if len(files) != 5 {
		t.Errorf("len(files) = %d, want 5", len(files))
	}

	// Root session should be session 1
	if root == nil {
		t.Fatal("root session is nil")
	}
	if root.ID != 1 {
		t.Errorf("root.ID = %d, want 1", root.ID)
	}

	// Verify all files are collected
	fileSet := make(map[string]bool)
	for _, f := range files {
		fileSet[f] = true
	}
	expected := []string{"a.go", "b.go", "c.go", "d.go", "e.go"}
	for _, f := range expected {
		if !fileSet[f] {
			t.Errorf("file %q not found in collected files", f)
		}
	}
}

func TestCollectReviewedFilesSingleSession(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create single session (no chain)
	session := &Session{
		BaseCommit: "abc123",
		HeadCommit: "def456",
		Status:     StatusCompleted,
		Files:      []string{"main.go", "util.go"},
	}
	if err := store.Create(session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	files, root, err := store.CollectReviewedFiles(1)
	if err != nil {
		t.Fatalf("CollectReviewedFiles(1) error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("len(files) = %d, want 2", len(files))
	}

	// Root should be the same session
	if root == nil || root.ID != 1 {
		t.Error("root should be session 1")
	}
}

func TestCollectReviewedFilesNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	_, _, err = store.CollectReviewedFiles(99)
	if err == nil {
		t.Error("CollectReviewedFiles(99) should error for non-existent session")
	}
}

func TestSessionWithFindings(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore("/test/project", tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	session := &Session{
		BaseCommit:    "abc123",
		FilesReviewed: 10,
		Status:        StatusCompleted,
		Findings: []Finding{
			{
				File:         "main.go",
				Line:         42,
				Severity:     "error",
				Category:     "security",
				Description:  "SQL injection vulnerability",
				SuggestedFix: "Use parameterized queries",
			},
			{
				File:        "handler.go",
				Line:        100,
				Severity:    "warning",
				Category:    "performance",
				Description: "N+1 query detected",
			},
		},
	}

	if err := store.Create(session); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := store.Load(1)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(loaded.Findings) != 2 {
		t.Errorf("len(Findings) = %d, want 2", len(loaded.Findings))
	}

	if loaded.Findings[0].Severity != "error" {
		t.Errorf("Findings[0].Severity = %q, want %q", loaded.Findings[0].Severity, "error")
	}
}

func TestHashPath(t *testing.T) {
	h1 := hashPath("/path/to/project1")
	h2 := hashPath("/path/to/project2")

	if h1 == h2 {
		t.Error("different paths should have different hashes")
	}

	// Same path should produce same hash
	h3 := hashPath("/path/to/project1")
	if h1 != h3 {
		t.Error("same path should have same hash")
	}

	// Hash should be 16 chars (8 bytes hex encoded)
	if len(h1) != 16 {
		t.Errorf("hash length = %d, want 16", len(h1))
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	content := []byte("test content")

	if err := atomicWrite(path, content); err != nil {
		t.Fatalf("atomicWrite() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	if string(data) != string(content) {
		t.Errorf("content = %q, want %q", string(data), string(content))
	}
}

func TestStatusConstants(t *testing.T) {
	// Verify status constants have expected values
	if StatusPending != "pending" {
		t.Errorf("StatusPending = %q, want %q", StatusPending, "pending")
	}
	if StatusInProgress != "in_progress" {
		t.Errorf("StatusInProgress = %q, want %q", StatusInProgress, "in_progress")
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %q, want %q", StatusCompleted, "completed")
	}
}
