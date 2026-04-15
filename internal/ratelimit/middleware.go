package ratelimit

import (
	"context"
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// AllowFunc is the signature matching Limiter.Allow, used to decouple
// middleware from the concrete Limiter for testing.
type AllowFunc func(ctx context.Context, key string, rate float64, burst int) (bool, int, time.Duration, error)

// UserIDExtractor returns the authenticated user's string ID from the request
// context. Returns ("", false) for anonymous requests. This avoids coupling
// the ratelimit package to the auth middleware's context key type.
type UserIDExtractor func(ctx context.Context) (string, bool)

// IPMiddleware returns middleware that rate-limits requests by client IP.
func IPMiddleware(allow AllowFunc, ratePerMin int, burst int) func(http.Handler) http.Handler {
	ratePerSec := float64(ratePerMin) / 60.0
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			key := "rl:ip:" + ip

			allowed, remaining, retryAfter, err := allow(r.Context(), key, ratePerSec, burst)
			if err != nil {
				// If Redis is down, let the request through.
				next.ServeHTTP(w, r)
				return
			}
			writeRateLimitHeaders(w, burst, remaining)
			if !allowed {
				writeLimited(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserMiddleware returns middleware that rate-limits requests by authenticated
// user ID. If the extractor returns no user (anonymous request), it passes through.
func UserMiddleware(allow AllowFunc, extract UserIDExtractor, ratePerMin int, burst int) func(http.Handler) http.Handler {
	ratePerSec := float64(ratePerMin) / 60.0
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			uid, ok := extract(r.Context())
			if !ok || uid == "" {
				next.ServeHTTP(w, r)
				return
			}
			key := "rl:user:" + uid

			allowed, remaining, retryAfter, err := allow(r.Context(), key, ratePerSec, burst)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			writeRateLimitHeaders(w, burst, remaining)
			if !allowed {
				writeLimited(w, retryAfter)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeRateLimitHeaders(w http.ResponseWriter, limit int, remaining int) {
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
}

func writeLimited(w http.ResponseWriter, retryAfter time.Duration) {
	secs := int(math.Ceil(retryAfter.Seconds()))
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]string{"error": "rate limit exceeded"})
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
