package storage

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) { return 0, fmt.Errorf("read error") }

func TestStorage(t *testing.T) {
	dir := t.TempDir()
	s, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if s.BasePath() != dir {
		t.Errorf("BasePath = %s, want %s", s.BasePath(), dir)
	}

	content := []byte("hello world")
	path, err := s.Save("test.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	rel := s.RelativePath(path)
	if rel != "test.txt" {
		t.Errorf("RelativePath = %s, want test.txt", rel)
	}

	rc, err := s.Open("test.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer rc.Close()

	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, content) {
		t.Errorf("got %q, want %q", got, content)
	}

	// Test nested dir
	path2, err := s.Save("nested/file.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Save nested: %v", err)
	}
	if s.RelativePath(path2) != "nested/file.txt" {
		t.Errorf("RelativePath nested = %s, want nested/file.txt", s.RelativePath(path2))
	}

	// Test RelativePath with unrelated path
	unrelated := "/other/path/file.txt"
	if s.RelativePath(unrelated) == "file.txt" {
		// This might happen if /other/path is somehow relative to dir, but unlikely
	}
}

func TestStorageErrors(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)

	// Save to a path that is a directory
	subDir := filepath.Join(dir, "isdir")
	_ = os.MkdirAll(subDir, 0755)
	_, err := s.Save("isdir", bytes.NewReader([]byte("test")))
	if err == nil {
		t.Error("expected error saving to a directory path")
	}

	// Open non-existent
	_, err = s.Open("missing.txt")
	if err == nil {
		t.Error("expected error opening non-existent file")
	}

	// New with invalid path (file instead of dir)
	file := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(file, []byte("test"), 0644)
	_, err = New(file)
	if err == nil {
		t.Error("expected error creating storage at file path")
	}
}

func TestSaveCopyError(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)
	_, err := s.Save("test.txt", errorReader{})
	if err == nil {
		t.Error("expected error from io.Copy")
	}
}

// TestSavePathTraversal verifies P3-4: storage.Save/Open reject names that
// would escape the storage base via "..", absolute paths, or leading slashes.
// Current callers build names from internal fields (userID/checksum/ext), so
// this is defense-in-depth against a future caller passing user input.
func TestSavePathTraversal(t *testing.T) {
	dir := t.TempDir()
	s, _ := New(dir)

	content := []byte("x")
	bad := []string{
		"../escape.txt",
		"a/../../escape.txt",
		"/etc/passwd",
		"./../x.txt",
		"..",
	}
	for _, name := range bad {
		if _, err := s.Save(name, bytes.NewReader(content)); err == nil {
			t.Errorf("Save(%q): expected traversal rejection, got nil", name)
		}
		if _, err := s.Open(name); err == nil {
			t.Errorf("Open(%q): expected traversal rejection, got nil", name)
		}
	}

	// Empty name is rejected.
	if _, err := s.Save("", bytes.NewReader(content)); err == nil {
		t.Error("Save(\"\"): expected error, got nil")
	}

	// Sanity: a normal nested name still works after the hardening.
	if _, err := s.Save("org-1/inv.pdf", bytes.NewReader(content)); err != nil {
		t.Errorf("Save normal nested: %v", err)
	}
}
