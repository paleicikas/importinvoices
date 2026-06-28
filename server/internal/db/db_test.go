package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	if s.DB() == nil {
		t.Fatal("DB() returned nil")
	}

	// Test Migrate
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// Test Migrate idempotency
	if err := s.Migrate(); err != nil {
		t.Fatalf("Migrate again: %v", err)
	}
}

func TestMigrateError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	s, _ := Open(path)
	
	// Close DB to cause error in Migrate
	_ = s.Close()
	if err := s.Migrate(); err == nil {
		t.Error("expected error migrating closed db")
	}
}

func TestOpenDirError(t *testing.T) {
	dir := t.TempDir()
	_, err := Open(dir)
	if err == nil {
		t.Error("expected error opening directory as db")
	}
}

func TestOpenMkdirError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(file, []byte("test"), 0644)

	_, err := Open(filepath.Join(file, "test.db"))
	if err == nil {
		t.Error("expected error creating db dir")
	}
}
