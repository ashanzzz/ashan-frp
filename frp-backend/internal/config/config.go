package config

import (
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	AppName           string
	Version           string
	HTTPAddr          string
	DataDir           string
	StateFile         string
	BaseDomain        string
	DatabaseDSN       string
	BootstrapUsername string
	BootstrapPassword string
	EncryptionKey     string
	APIBasePath       string
	UIBasePath        string
	DocsBasePath      string
	LogLevel          string
	LogFileEnabled    bool
	LogFilePath       string
	LogMaxSizeMB      int
	LogMaxBackups     int
	LogRetentionDays  int
	LogCompress       bool
}

func Load() Config {
	dataDir := getenv("DATA_DIR", "./data")
	return Config{
		AppName:           getenv("APP_NAME", "Ashan FRP"),
		Version:           getenv("APP_VERSION", "0.1.0"),
		HTTPAddr:          getenv("HTTP_ADDR", ":8080"),
		DataDir:           dataDir,
		StateFile:         getenv("STATE_FILE", filepath.Join(dataDir, "state.json")),
		BaseDomain:        getenv("BASE_DOMAIN", "335356119.xyz"),
		DatabaseDSN:       getenv("DATABASE_DSN", "file:"+filepath.Join(dataDir, "state.db")),
		BootstrapUsername: getenv("BOOTSTRAP_USERNAME", "admin"),
		BootstrapPassword: getenv("BOOTSTRAP_PASSWORD", "admin123"),
		EncryptionKey:     getenv("ENCRYPTION_KEY", ""),
		APIBasePath:       getenv("API_BASE_PATH", "/api/v1"),
		UIBasePath:        getenv("UI_BASE_PATH", "/ui"),
		DocsBasePath:      getenv("DOCS_BASE_PATH", "/api/docs"),
		LogLevel:          getenv("LOG_LEVEL", "info"),
		LogFileEnabled:    getenvBool("LOG_FILE_ENABLED", true),
		LogFilePath:       getenv("LOG_FILE_PATH", filepath.Join(dataDir, "logs", "ashan-frp.jsonl")),
		LogMaxSizeMB:      getenvInt("LOG_MAX_SIZE_MB", 20),
		LogMaxBackups:     getenvInt("LOG_MAX_BACKUPS", 20),
		LogRetentionDays:  getenvInt("LOG_RETENTION_DAYS", 30),
		LogCompress:       getenvBool("LOG_COMPRESS", true),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}
