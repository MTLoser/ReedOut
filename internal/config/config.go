package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	ListenAddr   string
	DatabasePath string
	DataDir      string
	TemplatePath string
	SecretKey    string
	DefaultUser  string
	DefaultPass  string
}

func Load() (*Config, error) {
	dataDir := envOr("REEDOUT_DATA_DIR", "./data")
	// Docker bind mounts require absolute paths
	dataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	return &Config{
		ListenAddr:   envOr("REEDOUT_LISTEN", ":8080"),
		DatabasePath: envOr("REEDOUT_DB", filepath.Join(dataDir, "reedout.db")),
		DataDir:      dataDir,
		TemplatePath: envOr("REEDOUT_TEMPLATES", "./templates"),
		SecretKey:    envOr("REEDOUT_SECRET", "change-me-in-production"),
		DefaultUser:  envOr("REEDOUT_DEFAULT_USER", "admin"),
		DefaultPass:  envOr("REEDOUT_DEFAULT_PASS", "admin"),
	}, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
