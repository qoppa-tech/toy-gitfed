package logger

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestDevHandler_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	h := newDevHandler(&buf, slog.LevelDebug)

	fixed := time.Date(2026, 4, 11, 14, 30, 0, 0, time.UTC)
	r := slog.NewRecord(fixed, slog.LevelInfo, "test message", 0)
	r.AddAttrs(slog.String("key", "value"))
	h.Handle(context.Background(), r)

	out := buf.String()
	if !strings.Contains(out, "2026-04-11 14:30:00") {
		t.Errorf("missing timestamp in %q", out)
	}
	if !strings.Contains(out, "INFO") {
		t.Errorf("missing level in %q", out)
	}
	if !strings.Contains(out, "test message") {
		t.Errorf("missing message in %q", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Errorf("missing attr in %q", out)
	}
}

func TestDevHandler_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	h := newDevHandler(&buf, slog.LevelWarn)

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "should be filtered", 0)
	h.Handle(context.Background(), r)

	if buf.Len() != 0 {
		t.Errorf("INFO should be filtered at WARN level, got %q", buf.String())
	}
}

func TestDevHandler_WithAttrs(t *testing.T) {
	var buf bytes.Buffer
	h := newDevHandler(&buf, slog.LevelDebug)
	h2 := h.WithAttrs([]slog.Attr{slog.String("request_id", "abc123")})

	r := slog.NewRecord(time.Now(), slog.LevelInfo, "hello", 0)
	h2.Handle(context.Background(), r)

	out := buf.String()
	if !strings.Contains(out, "request_id=abc123") {
		t.Errorf("missing pre-set attr in %q", out)
	}
}

func TestDevHandler_ColorPerLevel(t *testing.T) {
	levels := []struct {
		level slog.Level
		color string
	}{
		{slog.LevelDebug, "\033[36m"}, // cyan
		{slog.LevelInfo, "\033[32m"},  // green
		{slog.LevelWarn, "\033[33m"},  // yellow
		{slog.LevelError, "\033[31m"}, // red
	}

	for _, tc := range levels {
		var buf bytes.Buffer
		h := newDevHandler(&buf, slog.LevelDebug)
		r := slog.NewRecord(time.Now(), tc.level, "msg", 0)
		h.Handle(context.Background(), r)

		if !strings.Contains(buf.String(), tc.color) {
			t.Errorf("level %s: expected color %q in output %q", tc.level, tc.color, buf.String())
		}
	}
}
