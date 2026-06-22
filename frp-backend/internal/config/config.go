package config

import (
    "os"
    "path/filepath"
    "strconv"
)

type Config struct {
    AppName      string
    Version      string
    HTTPAddr     string
    DataDir      string
    StateFile    string
    APIBasePath  string
    UIBasePath   string
    DocsBasePath string
}

func Load() Config {
    dataDir := getenv("DATA_DIR", "./data")
    return Config{
        AppName:      getenv("APP_NAME", "Ashan FRP"),
        Version:      getenv("APP_VERSION", "dev"),
        HTTPAddr:     getenv("HTTP_ADDR", ":8080"),
        DataDir:      dataDir,
        StateFile:    getenv("STATE_FILE", filepath.Join(dataDir, "state.json")),
        APIBasePath:  getenv("API_BASE_PATH", "/api/v1"),
        UIBasePath:   getenv("UI_BASE_PATH", "/ui"),
        DocsBasePath: getenv("DOCS_BASE_PATH", "/api/docs"),
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
