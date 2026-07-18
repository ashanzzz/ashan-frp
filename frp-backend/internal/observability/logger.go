package observability

import (
	"compress/gzip"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Level         string
	FileEnabled   bool
	FilePath      string
	MaxSizeMB     int
	MaxBackups    int
	RetentionDays int
	Compress      bool
}

type rotatingWriter struct {
	mu   sync.Mutex
	cfg  Config
	file *os.File
	size int64
}

func NewLogger(cfg Config) (*slog.Logger, io.Closer, error) {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	writers := []io.Writer{os.Stdout}
	closers := []io.Closer{}
	if cfg.FileEnabled {
		writer, err := newRotatingWriter(cfg)
		if err != nil {
			return nil, nil, err
		}
		writers = append(writers, writer)
		closers = append(closers, writer)
	}
	handler := slog.NewJSONHandler(io.MultiWriter(writers...), &slog.HandlerOptions{Level: level, ReplaceAttr: redactAttr})
	logger := slog.New(handler)
	return logger, multiCloser(closers), nil
}

type multiCloser []io.Closer

func (m multiCloser) Close() error {
	for _, c := range m {
		_ = c.Close()
	}
	return nil
}

func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if attr.Key == slog.TimeKey {
		return slog.Time("timestamp", attr.Value.Time().UTC())
	}
	key := strings.ToLower(attr.Key)
	for _, term := range []string{"password", "authorization", "cookie", "encrypted", "secret"} {
		if strings.Contains(key, term) {
			return slog.String(attr.Key, "[REDACTED]")
		}
	}
	if strings.Contains(key, "token") && key != "token_mask" {
		return slog.String(attr.Key, "[REDACTED]")
	}
	if strings.Contains(key, "credential") && key != "credential_ref" && key != "credential_revision" {
		return slog.String(attr.Key, "[REDACTED]")
	}
	return attr
}

func TokenMask(secret string) string {
	secret = strings.TrimSpace(secret)
	if len(secret) <= 4 {
		return "****"
	}
	return "****" + secret[len(secret)-4:]
}
func CredentialRef(secret string, key []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(secret))
	sum := hex.EncodeToString(mac.Sum(nil))
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}
func RequestLogger(ctx context.Context, event string, attrs ...any) {
	slog.Default().InfoContext(ctx, event, attrs...)
}

func newRotatingWriter(cfg Config) (*rotatingWriter, error) {
	if cfg.MaxSizeMB <= 0 {
		cfg.MaxSizeMB = 20
	}
	if cfg.MaxBackups <= 0 {
		cfg.MaxBackups = 20
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 30
	}
	if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0700); err != nil {
		return nil, err
	}
	w := &rotatingWriter{cfg: cfg}
	if err := w.open(); err != nil {
		return nil, err
	}
	_ = os.Chmod(cfg.FilePath, 0600)
	go w.cleanup()
	return w, nil
}
func (w *rotatingWriter) open() error {
	file, err := os.OpenFile(w.cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, _ := file.Stat()
	w.file = file
	if info != nil {
		w.size = info.Size()
	}
	return nil
}
func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > int64(w.cfg.MaxSizeMB)*1024*1024 {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}
func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}
func (w *rotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
	}
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	rotated := w.cfg.FilePath + "." + stamp
	if err := os.Rename(w.cfg.FilePath, rotated); err != nil && !os.IsNotExist(err) {
		return err
	}
	if w.cfg.Compress {
		go gzipFile(rotated)
	}
	w.size = 0
	if err := w.open(); err != nil {
		return err
	}
	go w.cleanup()
	return nil
}
func gzipFile(path string) {
	in, err := os.Open(path)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(path + ".gz")
	if err != nil {
		return
	}
	gz := gzip.NewWriter(out)
	_, copyErr := io.Copy(gz, in)
	closeErr := gz.Close()
	_ = out.Close()
	if copyErr == nil && closeErr == nil {
		_ = os.Remove(path)
	}
}
func (w *rotatingWriter) cleanup() {
	pattern := w.cfg.FilePath + ".*"
	files, _ := filepath.Glob(pattern)
	sort.Slice(files, func(i, j int) bool {
		a, _ := os.Stat(files[i])
		b, _ := os.Stat(files[j])
		if a == nil || b == nil {
			return files[i] > files[j]
		}
		return a.ModTime().After(b.ModTime())
	})
	cutoff := time.Now().AddDate(0, 0, -w.cfg.RetentionDays)
	for i, path := range files {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if i >= w.cfg.MaxBackups || info.ModTime().Before(cutoff) {
			_ = os.Remove(path)
		}
	}
}
