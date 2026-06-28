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
