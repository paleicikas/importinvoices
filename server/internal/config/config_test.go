package config

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestConfig(t *testing.T) {
	dir := t.TempDir()

	// Test Resolve new config
	cfg, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg.DataDir != dir {
		t.Errorf("DataDir = %s, want %s", cfg.DataDir, dir)
	}
	if cfg.DBPath != filepath.Join(dir, "data.db") {
		t.Errorf("DBPath = %s", cfg.DBPath)
	}

	// Test Save
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Test Load
	cfg2, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg2.HTTPAddr != cfg.HTTPAddr {
		t.Errorf("HTTPAddr = %s, want %s", cfg2.HTTPAddr, cfg.HTTPAddr)
	}

	// Test Resolve existing
	cfg3, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve existing: %v", err)
	}
	if cfg3.HTTPAddr != cfg.HTTPAddr {
		t.Errorf("HTTPAddr = %s, want %s", cfg3.HTTPAddr, cfg.HTTPAddr)
	}

	// Test DefaultDataDir
	_, err = DefaultDataDir()
	if err != nil {
		t.Errorf("DefaultDataDir: %v", err)
	}
}

func TestResolveEmptyDataDir(t *testing.T) {
	// Should use DefaultDataDir
	cfg, err := Resolve("")
	if err != nil {
		t.Fatalf("Resolve empty: %v", err)
	}
	if cfg.DataDir == "" {
		t.Error("DataDir is empty")
	}
}

func TestLoadCorrupt(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(ConfigPath(dir), []byte("invalid json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error loading corrupt config")
	}
}

func TestLoadReadError(t *testing.T) {
	dir := t.TempDir()
	_ = os.MkdirAll(ConfigPath(dir), 0755) // config.json is a dir

	_, err := Load(dir)
	if err == nil {
		t.Error("expected error loading config from dir")
	}
}

func TestSaveMkdirError(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(file, []byte("test"), 0644)

	cfg := &Config{DataDir: filepath.Join(file, "subdir")}
	err := Save(cfg)
	if err == nil {
		t.Error("expected error saving config to invalid path")
	}
}

func TestResolvePartialConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{
		DataDir: dir,
		// Other fields empty
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg2, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg2.DBPath == "" {
		t.Error("DBPath should be filled")
	}
	if cfg2.HTTPAddr == "" {
		t.Error("HTTPAddr should be filled")
	}
}

func TestDefaultDataDirError(t *testing.T) {
	oldHome := os.Getenv("HOME")
	oldUserProfile := os.Getenv("USERPROFILE")
	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")
	defer func() {
		if oldHome != "" {
			os.Setenv("HOME", oldHome)
		}
		if oldUserProfile != "" {
			os.Setenv("USERPROFILE", oldUserProfile)
		}
	}()

	_, err := DefaultDataDir()
	if err == nil {
		t.Error("expected error from DefaultDataDir with no home")
	}
}

func TestResolvePortConflict(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("failed to listen on random port")
	}
	defer ln.Close()
	addr := ln.Addr().String()

	dir := t.TempDir()
	cfg := &Config{
		DataDir:  dir,
		HTTPAddr: addr,
	}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cfg2, err := Resolve(dir)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if cfg2.HTTPAddr == addr {
		t.Errorf("expected different port than %s", addr)
	}
}
