package logger

import (
	"log/slog"
	"os"
	"strings"
)

type defaultLogger struct {
	slog *slog.Logger
}

// New creates a Logger from the given Config.
func New(cfg Config) Logger {
	level := parseLevel(cfg.Level)
	var handler slog.Handler
	if strings.EqualFold(cfg.Env, "PROD") {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = newDevHandler(os.Stdout, level)
	}
	return &defaultLogger{slog: slog.New(handler)}
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
