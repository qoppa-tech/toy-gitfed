package logger

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// Compile-time check: defaultLogger implements Logger.
var _ Logger = (*defaultLogger)(nil)

func newTestLogger(buf *bytes.Buffer, env, level string) Logger {
	cfg := Config{Env: env, Level: level}
	lvl := parseLevel(cfg.Level)
	var handler slog.Handler
	if strings.EqualFold(cfg.Env, "PROD") {
		handler = slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: lvl})
	} else {
		handler = newDevHandler(buf, lvl)
	}
	return &defaultLogger{slog: slog.New(handler)}
}

func TestNew_ReturnsLogger(t *testing.T) {
	l := New(Config{Env: "DEV", Level: "info"})
	if l == nil {
		t.Fatal("New returned nil")
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "DEV", "warn")

	l.Info("should not appear")
	if buf.Len() != 0 {
		t.Errorf("INFO logged at WARN level: %q", buf.String())
	}

	l.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("WARN not logged at WARN level")
	}
}

func TestLogger_ProdJSON(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	l.Info("hello", "key", "value")

	var entry map[string]any
	if err := json.NewDecoder(&buf).Decode(&entry); err != nil {
		t.Fatalf("JSON decode: %v", err)
	}
	if entry["msg"] != "hello" {
		t.Errorf("msg = %v, want hello", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %v, want value", entry["key"])
	}
	if entry["level"] != "INFO" {
		t.Errorf("level = %v, want INFO", entry["level"])
	}
}

func TestLogger_With(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	l2 := l.With("request_id", "abc123")
	l2.Info("test")

	var entry map[string]any
	json.NewDecoder(&buf).Decode(&entry)
	if entry["request_id"] != "abc123" {
		t.Errorf("request_id = %v, want abc123", entry["request_id"])
	}
}

func TestLogger_WithImmutable(t *testing.T) {
	var buf bytes.Buffer
	l := newTestLogger(&buf, "PROD", "info")

	_ = l.With("extra", "field")
	l.Info("original")

	var entry map[string]any
	json.NewDecoder(&buf).Decode(&entry)
	if _, ok := entry["extra"]; ok {
		t.Error("With mutated original logger")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"invalid", slog.LevelInfo},
	}
	for _, tc := range tests {
		got := parseLevel(tc.input)
		if got != tc.want {
			t.Errorf("parseLevel(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
