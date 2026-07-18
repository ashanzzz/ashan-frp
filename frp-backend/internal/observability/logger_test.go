package observability

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoggerWritesJSONAndRedactsSecrets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "ashan-frp.jsonl")
	logger, closer, err := NewLogger(Config{Level: "debug", FileEnabled: true, FilePath: path, MaxSizeMB: 1, MaxBackups: 2, RetentionDays: 30, Compress: true})
	require.NoError(t, err)
	secret := "known-secret-token-value"
	logger.Info("credential.test", "password", secret, "api_token", secret, "authorization", "Bearer "+secret, "token_mask", "****alue", "credential_ref", "abc123def456", "credential", secret)
	require.NoError(t, closer.Close())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(content)
	require.NotContains(t, text, secret)
	require.Contains(t, text, "[REDACTED]")
	require.Contains(t, text, "****alue")
	require.Contains(t, text, "abc123def456")
	var entry map[string]any
	require.NoError(t, json.Unmarshal(content, &entry))
	require.NotEmpty(t, entry["timestamp"])
	require.NotContains(t, entry, "time")
}

func TestRotatingWriterRotatesLargeFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ashan-frp.jsonl")
	writer, err := newRotatingWriter(Config{FilePath: path, MaxSizeMB: 1, MaxBackups: 2, RetentionDays: 30, Compress: false})
	require.NoError(t, err)
	payload := []byte(strings.Repeat("x", 700*1024))
	_, err = writer.Write(payload)
	require.NoError(t, err)
	_, err = writer.Write(payload)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	rotated, err := filepath.Glob(path + ".*")
	require.NoError(t, err)
	require.NotEmpty(t, rotated)
}

func TestCredentialIdentityIsStableWithoutRevealingSecret(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	first := CredentialRef("same-token", key)
	require.Equal(t, first, CredentialRef("same-token", key))
	require.NotEqual(t, first, CredentialRef("different-token", key))
	require.Len(t, first, 12)
	require.Equal(t, "****oken", TokenMask("same-token"))
	_ = slog.LevelInfo
}
