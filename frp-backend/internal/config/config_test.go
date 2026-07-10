package config

import (
	"path/filepath"
	"testing"
)

func TestLoad_usesDefaults_whenEnvUnset(t *testing.T) {
	clearConfigEnv(t)

	cfg := Load()

	assertConfig(t, cfg, Config{
		AppName:           "Ashan FRP",
		Version:           "0.1.0",
		HTTPAddr:          ":8080",
		DataDir:           "./data",
		StateFile:         filepath.Join("./data", "state.json"),
		BaseDomain:        "335356119.xyz",
		DatabaseDSN:       "file:" + filepath.Join("./data", "state.db"),
		BootstrapUsername:  "admin",
		BootstrapPassword:  "admin123",
		APIBasePath:       "/api/v1",
		UIBasePath:        "/ui",
		DocsBasePath:      "/api/docs",
	})
}

func TestLoad_overridesEnvValues(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("APP_NAME", "Ashan FRP Pro")
	t.Setenv("APP_VERSION", "1.2.3")
	t.Setenv("HTTP_ADDR", "127.0.0.1:18080")
	t.Setenv("DATA_DIR", dataDir)
	t.Setenv("STATE_FILE", "./custom/state.json")
	t.Setenv("BASE_DOMAIN", "example.test")
	t.Setenv("DATABASE_DSN", "file:/tmp/ashan.db")
	t.Setenv("BOOTSTRAP_USERNAME", "root")
	t.Setenv("BOOTSTRAP_PASSWORD", "secret")
	t.Setenv("API_BASE_PATH", "/api/v2")
	t.Setenv("UI_BASE_PATH", "/app")
	t.Setenv("DOCS_BASE_PATH", "/docs")

	cfg := Load()

	assertConfig(t, cfg, Config{
		AppName:           "Ashan FRP Pro",
		Version:           "1.2.3",
		HTTPAddr:          "127.0.0.1:18080",
		DataDir:           dataDir,
		StateFile:         "./custom/state.json",
		BaseDomain:        "example.test",
		DatabaseDSN:       "file:/tmp/ashan.db",
		BootstrapUsername:  "root",
		BootstrapPassword:  "secret",
		APIBasePath:       "/api/v2",
		UIBasePath:        "/app",
		DocsBasePath:      "/docs",
	})
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"APP_NAME",
		"APP_VERSION",
		"HTTP_ADDR",
		"DATA_DIR",
		"STATE_FILE",
		"BASE_DOMAIN",
		"DATABASE_DSN",
		"BOOTSTRAP_USERNAME",
		"BOOTSTRAP_PASSWORD",
		"API_BASE_PATH",
		"UI_BASE_PATH",
		"DOCS_BASE_PATH",
	} {
		t.Setenv(key, "")
	}
}

func assertConfig(t *testing.T, got, want Config) {
	t.Helper()
	if got != want {
		t.Fatalf("unexpected config\n got: %#v\nwant: %#v", got, want)
	}
}
