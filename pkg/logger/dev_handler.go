package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

const (
	colorReset  = "\033[0m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorGray   = "\033[90m"
)

// devHandler writes colorized, human-readable log lines.
type devHandler struct {
	w     io.Writer
	mu    *sync.Mutex
	level slog.Level
	attrs []slog.Attr
}

func newDevHandler(w io.Writer, level slog.Level) slog.Handler {
	return &devHandler{w: w, mu: &sync.Mutex{}, level: level}
}

func (h *devHandler) Enabled(_ context.Context, l slog.Level) bool {
	return l >= h.level
}

func (h *devHandler) Handle(_ context.Context, r slog.Record) error {
	if !h.Enabled(context.Background(), r.Level) {
		return nil
	}

	ts := r.Time.Format(time.DateTime)
	lvl := levelColor(r.Level)

	h.mu.Lock()
	defer h.mu.Unlock()

	fmt.Fprintf(h.w, "%s%s%s %s%-5s%s %s",
		colorGray, ts, colorReset,
		lvl.color, lvl.label, colorReset,
		r.Message,
	)

	// Pre-set attrs from With.
	for _, a := range h.attrs {
		fmt.Fprintf(h.w, " %s%s=%v%s", colorCyan, a.Key, a.Value, colorReset)
	}

	// Record attrs.
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(h.w, " %s%s=%v%s", colorCyan, a.Key, a.Value, colorReset)
		return true
	})

	fmt.Fprintln(h.w)
	return nil
}

func (h *devHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, len(h.attrs), len(h.attrs)+len(attrs))
	copy(merged, h.attrs)
	merged = append(merged, attrs...)
	return &devHandler{w: h.w, mu: h.mu, level: h.level, attrs: merged}
}

func (h *devHandler) WithGroup(_ string) slog.Handler {
	return h // groups not used in this project
}

type levelStyle struct {
	color string
	label string
}

func levelColor(l slog.Level) levelStyle {
	switch {
	case l < slog.LevelInfo:
		return levelStyle{colorCyan, "DEBUG"}
	case l < slog.LevelWarn:
		return levelStyle{colorGreen, "INFO"}
	case l < slog.LevelError:
		return levelStyle{colorYellow, "WARN"}
	default:
		return levelStyle{colorRed, "ERROR"}
	}
}
