package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type contextKey string

const userIDKey contextKey = "user_id"

const (
	AccessCookieName  = "access_token"
	RefreshCookieName = "refresh_token"
)

// TokenValidator validates an access token and returns the associated user ID.
type TokenValidator interface {
	Validate(ctx context.Context, token string) (uuid.UUID, error)
}

// Auth returns middleware that validates access tokens and injects user ID into context.
func Auth(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractAccessToken(r)
			if token == "" {
				logger.FromContext(r.Context()).Info("auth token missing", "step", "token_extract")
				writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid token"})
				return
			}

			userID, err := validator.Validate(r.Context(), token)
			if err != nil {
				logger.FromContext(r.Context()).Info("auth token rejected", "step", "token_validate", "error", err)
				writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			reqLog := logger.FromContext(ctx).With("user_id", userID.String())
			ctx = logger.WithContext(ctx, reqLog)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext extracts the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	uid, ok := ctx.Value(userIDKey).(uuid.UUID)
	return uid, ok
}

func extractAccessToken(r *http.Request) string {
	if c, err := r.Cookie(AccessCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return ""
	}
	return token
}

// SetAccessCookie writes the access token as an HTTP-only, Secure, SameSite=Strict cookie.
func SetAccessCookie(w http.ResponseWriter, token string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// SetRefreshCookie writes the refresh token as an HTTP-only cookie scoped to /auth/refresh.
func SetRefreshCookie(w http.ResponseWriter, token string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    token,
		Path:     "/auth/refresh",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearAccessCookie expires the access token cookie.
func ClearAccessCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     AccessCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

// ClearRefreshCookie expires the refresh token cookie.
func ClearRefreshCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     RefreshCookieName,
		Value:    "",
		Path:     "/auth/refresh",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func writeJSON(ctx context.Context, w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logger.FromContext(ctx).Error("response json encode failed", "status", status, "error", err)
	}
}
