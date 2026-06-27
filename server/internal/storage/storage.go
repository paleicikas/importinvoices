package storage

import (
	"io"
	"os"
	"path/filepath"
)

type Storage struct {
	basePath string
}

func New(basePath string) (*Storage, error) {
	if err := os.MkdirAll(basePath, 0o755); err != nil {
		return nil, err
	}
	return &Storage{basePath: basePath}, nil
}

func (s *Storage) Save(name string, r io.Reader) (string, error) {
	path := filepath.Join(s.basePath, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}

	return path, nil
}

func (s *Storage) Open(name string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.basePath, name))
}

func (s *Storage) RelativePath(path string) string {
	rel, err := filepath.Rel(s.basePath, path)
	if err != nil {
		return path
	}
	// Normalize to forward slashes for web usage
	return filepath.ToSlash(rel)
}

func (s *Storage) BasePath() string {
	return s.basePath
}
