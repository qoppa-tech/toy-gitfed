package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type mockValidator struct {
	userID pgtype.UUID
	err    error
}

func (m *mockValidator) Validate(_ context.Context, _ string) (pgtype.UUID, error) {
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
	expectedUID := pgtype.UUID{
		Bytes: [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
			0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		Valid: true,
	}

	var gotUID pgtype.UUID
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
	expectedUID := pgtype.UUID{
		Bytes: [16]byte{0xaa, 0xbb, 0xcc, 0xdd, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
		Valid: true,
	}

	var gotUID pgtype.UUID

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

func (tc *tokenCapture) Validate(_ context.Context, token string) (pgtype.UUID, error) {
	tc.lastToken = token
	return pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, nil
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
		t.Errorf("Path = %q, want %q (refresh cookie must be path-restricted)", c.Path, "/auth/refresh")
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

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name, header, want string
	}{
		{"valid", "Bearer abc123", "abc123"},
		{"no prefix", "abc123", ""},
		{"empty", "", ""},
		{"basic auth", "Basic dXNlcjpwYXNz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}
			if got := extractBearerToken(r); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
