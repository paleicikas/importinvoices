package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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

// resolveSafe turns a caller-supplied relative name into an absolute filesystem
// path inside basePath. It rejects path-traversal and absolute-path attempts
// (`..`, leading `/`, Windows drive letters) so a future caller passing user
// input cannot escape the storage root. The returned path is guaranteed to be
// basePath or a descendant of it.
func (s *Storage) resolveSafe(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("storage: empty name")
	}
	// Reject absolute paths (Unix leading "/", Windows "C:\" or "\\").
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "\\") {
		return "", fmt.Errorf("storage: absolute path not allowed: %q", name)
	}
	// Reject Windows drive-letter paths like "C:\...".
	if len(name) >= 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z')) {
		return "", fmt.Errorf("storage: absolute path not allowed: %q", name)
	}
	// Reject any ".." component, no matter where it appears.
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return "", fmt.Errorf("storage: path traversal not allowed: %q", name)
		}
	}
	base, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", err
	}
	full, err := filepath.Abs(filepath.Join(base, filepath.FromSlash(name)))
	if err != nil {
		return "", err
	}
	if full != base && !strings.HasPrefix(full, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("storage: resolved path escapes base: %q", name)
	}
	return full, nil
}

func (s *Storage) Save(name string, r io.Reader) (string, error) {
	path, err := s.resolveSafe(name)
	if err != nil {
		return "", err
	}
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
	path, err := s.resolveSafe(name)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
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
