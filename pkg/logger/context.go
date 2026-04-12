package logger

import (
	"context"
	"log/slog"
	"os"
)

type ctxKey struct{}

// globalDefault is the fallback logger when no logger exists in the context.
var globalDefault Logger = &defaultLogger{slog: slog.New(slog.NewTextHandler(os.Stderr, nil))}

// SetDefault replaces the global fallback logger. Call this once at startup.
func SetDefault(l Logger) {
	globalDefault = l
}

// WithContext returns a new context carrying the given Logger.
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the Logger stored in ctx, or the global default.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}
	return globalDefault
}
