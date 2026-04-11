package logger

import (
	"io"
	"log/slog"
)

func newDevHandler(w io.Writer, level slog.Level) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
}
