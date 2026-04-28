package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/internal/ratelimit"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type mockValidator struct {
	userID uuid.UUID
	err    error
}

func (m *mockValidator) Validate(_ context.Context, _ string) (uuid.UUID, error) {
	return m.userID, m.err
}

func TestAuth_MissingToken(t *testing.T) {
	mw := Auth(&mockValidator{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "missing or invalid token" {
		t.Errorf("error = %q, want %q", body["error"], "missing or invalid token")
	}
}

func TestAuth_InvalidToken(t *testing.T) {
	mw := Auth(&mockValidator{err: context.DeadlineExceeded})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAuth_ValidBearerToken(t *testing.T) {
	expectedUID := uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10")

	var gotUID uuid.UUID
	var gotOK bool

	mw := Auth(&mockValidator{userID: expectedUID})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, gotOK = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !gotOK {
		t.Fatal("UserIDFromContext should return ok=true")
	}
	if gotUID != expectedUID {
		t.Errorf("UserID = %v, want %v", gotUID, expectedUID)
	}
}

func TestAuth_ValidAccessCookie(t *testing.T) {
	expectedUID := uuid.MustParse("aabbccdd-0000-0000-0000-000000000001")

	var gotUID uuid.UUID

	mw := Auth(&mockValidator{userID: expectedUID})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "cookie-token"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if gotUID != expectedUID {
		t.Errorf("UserID = %v, want %v", gotUID, expectedUID)
	}
}

func TestAuth_CookieTakesPrecedenceOverBearer(t *testing.T) {
	validator := &tokenCapture{}
	mw := Auth(validator)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "cookie-wins"})
	req.Header.Set("Authorization", "Bearer bearer-loses")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if validator.lastToken != "cookie-wins" {
		t.Errorf("token = %q, want %q (cookie should take precedence)", validator.lastToken, "cookie-wins")
	}
}

type tokenCapture struct {
	lastToken string
}

func (tc *tokenCapture) Validate(_ context.Context, token string) (uuid.UUID, error) {
	tc.lastToken = token
	return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
}

func TestUserIDFromContext_Missing(t *testing.T) {
	_, ok := UserIDFromContext(context.Background())
	if ok {
		t.Error("UserIDFromContext should return ok=false for empty context")
	}
}

func TestSetAccessCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetAccessCookie(w, "test-token", 900, true)

	c := w.Result().Cookies()[0]
	if c.Name != AccessCookieName {
		t.Errorf("name = %q, want %q", c.Name, AccessCookieName)
	}
	if c.Value != "test-token" {
		t.Errorf("value = %q, want %q", c.Value, "test-token")
	}
	if !c.HttpOnly {
		t.Error("should be HttpOnly")
	}
	if !c.Secure {
		t.Error("should be Secure when secure=true")
	}
	if c.SameSite != http.SameSiteStrictMode {
		t.Errorf("SameSite = %v, want Strict", c.SameSite)
	}
	if c.MaxAge != 900 {
		t.Errorf("MaxAge = %d, want 900", c.MaxAge)
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want %q", c.Path, "/")
	}
}

func TestSetRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetRefreshCookie(w, "refresh-tok", 604800, true)

	c := w.Result().Cookies()[0]
	if c.Name != RefreshCookieName {
		t.Errorf("name = %q, want %q", c.Name, RefreshCookieName)
	}
	if c.Path != "/auth/refresh" {
		t.Errorf("Path = %q, want %q", c.Path, "/auth/refresh")
	}
	if !c.HttpOnly {
		t.Error("should be HttpOnly")
	}
	if c.MaxAge != 604800 {
		t.Errorf("MaxAge = %d, want 604800", c.MaxAge)
	}
}

func TestClearAccessCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearAccessCookie(w, true)

	c := w.Result().Cookies()[0]
	if c.Value != "" {
		t.Errorf("value = %q, want empty", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
}

func TestClearRefreshCookie(t *testing.T) {
	w := httptest.NewRecorder()
	ClearRefreshCookie(w, true)

	c := w.Result().Cookies()[0]
	if c.Name != RefreshCookieName {
		t.Errorf("name = %q, want %q", c.Name, RefreshCookieName)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
	if c.Path != "/auth/refresh" {
		t.Errorf("Path = %q, want %q", c.Path, "/auth/refresh")
	}
}

func TestExtractAccessToken_CookieFirst(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: AccessCookieName, Value: "from-cookie"})
	req.Header.Set("Authorization", "Bearer from-header")

	got := extractAccessToken(req)
	if got != "from-cookie" {
		t.Errorf("got %q, want %q (cookie first)", got, "from-cookie")
	}
}

func TestExtractAccessToken_FallbackToBearer(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer from-header")

	got := extractAccessToken(req)
	if got != "from-header" {
		t.Errorf("got %q, want %q", got, "from-header")
	}
}

func TestExtractAccessToken_NeitherSet(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if got := extractAccessToken(req); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestAuth_EnrichesLoggerWithUserID(t *testing.T) {
	expectedUID := uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10")

	var buf bytes.Buffer
	testLog := logger.NewWithWriter(&buf, logger.Config{Env: "PROD", Level: "debug"})

	var capturedLogger logger.Logger
	mw := Auth(&mockValidator{userID: expectedUID})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedLogger = logger.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	ctx := logger.WithContext(req.Context(), testLog)
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if capturedLogger == nil {
		t.Fatal("logger not found in context")
	}

	// Log something with the captured logger to verify user_id is present.
	capturedLogger.Info("check")
	var entry map[string]any
	json.NewDecoder(&buf).Decode(&entry)
	if entry["user_id"] != expectedUID.String() {
		t.Errorf("user_id = %v, want %s", entry["user_id"], expectedUID)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(t.Context(), w, http.StatusOK, map[string]string{"status": "ok"})

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body = %v, want status=ok", body)
	}
}

func TestAuthChainWithUserRateLimit(t *testing.T) {
	uid := uuid.MustParse("01020304-0506-0708-090a-0b0c0d0e0f10")

	calls := 0
	fakeAllow := func(_ context.Context, _ string, _ float64, _ int) (bool, int, time.Duration, error) {
		calls++
		if calls > 2 {
			return false, 0, time.Second, nil
		}
		return true, 2 - calls, 0, nil
	}

	extractor := func(ctx context.Context) (string, bool) {
		id, ok := UserIDFromContext(ctx)
		if !ok {
			return "", false
		}
		return id.String(), true
	}

	userRL := ratelimit.UserMiddleware(fakeAllow, extractor, 100, 2)
	authMw := Auth(&mockValidator{userID: uid})

	// Compose the chain exactly as http.go does: auth -> rate limit -> handler.
	authChain := func(next http.Handler) http.Handler {
		h := userRL(next)
		return authMw(h)
	}

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := authChain(inner)

	// Unauthenticated request: auth rejects before rate limiting.
	t.Run("no_token_returns_401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
		}
		if calls != 0 {
			t.Errorf("rate limiter called %d times, want 0 (auth should reject first)", calls)
		}
	})

	// Authenticated request: rate limit allows, headers present.
	t.Run("authenticated_request_has_rate_limit_headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if rl := w.Header().Get("X-RateLimit-Limit"); rl != "2" {
			t.Errorf("X-RateLimit-Limit = %q, want %q", rl, "2")
		}
		if rr := w.Header().Get("X-RateLimit-Remaining"); rr != "1" {
			t.Errorf("X-RateLimit-Remaining = %q, want %q", rr, "1")
		}
	})

	// Second authenticated request: still allowed, remaining decrements.
	t.Run("second_request_decrements_remaining", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if rr := w.Header().Get("X-RateLimit-Remaining"); rr != "0" {
			t.Errorf("X-RateLimit-Remaining = %q, want %q", rr, "0")
		}
	})

	// Third authenticated request: rate limited.
	t.Run("exhausted_returns_429", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
		}
		if ra := w.Header().Get("Retry-After"); ra != "1" {
			t.Errorf("Retry-After = %q, want %q", ra, "1")
		}

		var body map[string]string
		json.NewDecoder(w.Body).Decode(&body)
		if body["error"] != "rate limit exceeded" {
			t.Errorf("error = %q, want %q", body["error"], "rate limit exceeded")
		}
	})
}
