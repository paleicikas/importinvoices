package config

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

func isPortAvailable(addr string) bool {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func findAvailablePort(host string, start, end int) string {
	for port := start; port <= end; port++ {
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		if isPortAvailable(addr) {
			return addr
		}
	}
	return net.JoinHostPort(host, strconv.Itoa(start))
}

var Version = "1.2.0"

type Config struct {
	DataDir         string   `json:"data_dir"`
	DBPath          string   `json:"db_path"`
	HTTPAddr        string   `json:"http_addr"`
	StoragePath     string   `json:"storage_path"`
	MaxUploadBytes  int64    `json:"max_upload_bytes"`
	TrustedProxies  []string `json:"trusted_proxies,omitempty"`
}

func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".importinvoices"), nil
}

func ConfigPath(dataDir string) string {
	return filepath.Join(dataDir, "config.json")
}

func Load(dataDir string) (*Config, error) {
	path := ConfigPath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(cfg.DataDir), data, 0o600)
}

func Resolve(dataDir string) (*Config, error) {
	if dataDir == "" {
		var err error
		dataDir, err = DefaultDataDir()
		if err != nil {
			return nil, err
		}
	}
	cfg, err := Load(dataDir)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		cfg = &Config{
			DataDir:        dataDir,
			DBPath:         filepath.Join(dataDir, "data.db"),
			HTTPAddr:       findAvailablePort("127.0.0.1", 8080, 8088),
			MaxUploadBytes: 10485760, // 10MB
		}
	}
	if cfg.DataDir == "" {
		cfg.DataDir = dataDir
	}
	if cfg.DBPath == "" {
		cfg.DBPath = filepath.Join(cfg.DataDir, "data.db")
	}
	if cfg.HTTPAddr == "" || !isPortAvailable(cfg.HTTPAddr) {
		cfg.HTTPAddr = findAvailablePort("127.0.0.1", 8080, 8088)
	}
	if cfg.StoragePath == "" {
		cfg.StoragePath = filepath.Join(cfg.DataDir, "files")
	}
	if cfg.MaxUploadBytes == 0 {
		cfg.MaxUploadBytes = 10485760
	}
	return cfg, nil
}
