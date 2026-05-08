package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

type defaultLogger struct {
	slog *slog.Logger
}

// NewWithWriter creates a Logger writing to w. Useful for testing.
func NewWithWriter(w io.Writer, cfg Config) Logger {
	level := parseLevel(cfg.Level)
	var handler slog.Handler
	if strings.EqualFold(cfg.Env, "DEV") {
		handler = newDevHandler(w, level)
	} else {
		handler = slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	}
	return &defaultLogger{slog: slog.New(handler)}
}

// New creates a Logger from the given Config. Output goes to stdout.
func New(cfg Config) Logger {
	return NewWithWriter(os.Stdout, cfg)
}

func (l *defaultLogger) Debug(msg string, args ...any) { l.slog.Debug(msg, args...) }
func (l *defaultLogger) Info(msg string, args ...any)  { l.slog.Info(msg, args...) }
func (l *defaultLogger) Warn(msg string, args ...any)  { l.slog.Warn(msg, args...) }
func (l *defaultLogger) Error(msg string, args ...any) { l.slog.Error(msg, args...) }

func (l *defaultLogger) Fatal(msg string, args ...any) {
	l.slog.Error(msg, args...)
	os.Exit(1)
}

func (l *defaultLogger) With(key string, value any) Logger {
	return &defaultLogger{slog: l.slog.With(key, value)}
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
