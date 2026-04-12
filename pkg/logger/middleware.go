package logger

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Middleware returns HTTP middleware that creates a request-scoped logger.
// The logger is enriched with request_id, method, path, and client IP.
// It logs "request started" on entry and "request completed" with status
// and duration on exit.
func Middleware(log Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reqLog := log.
				With("request_id", uuid.New().String()).
				With("method", r.Method).
				With("path", r.URL.Path).
				With("ip", clientIP(r))

			reqLog.Info("request started")

			ctx := WithContext(r.Context(), reqLog)
			sw := &statusWriter{ResponseWriter: w}
			start := time.Now()

			next.ServeHTTP(sw, r.WithContext(ctx))

			reqLog.Info("request completed",
				"status", sw.status,
				"duration", time.Since(start).String(),
			)
		})
	}
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func (sw *statusWriter) Write(b []byte) (int, error) {
	if sw.status == 0 {
		sw.status = http.StatusOK
	}
	return sw.ResponseWriter.Write(b)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
