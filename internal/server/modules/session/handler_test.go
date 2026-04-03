package session

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qoppa-tech/toy-gitfed/internal/server/middleware"
)

func TestExtractBearerToken(t *testing.T) {
	tests := []struct {
		name, header, want string
	}{
		{"valid bearer token", "Bearer abc123", "abc123"},
		{"missing prefix", "abc123", ""},
		{"empty header", "", ""},
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

func TestExtractRefreshToken_CookieFirst(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: "cookie-refresh"})
	r.Header.Set("Authorization", "Bearer bearer-refresh")

	got := extractRefreshToken(r)
	if got != "cookie-refresh" {
		t.Errorf("got %q, want %q (cookie takes precedence)", got, "cookie-refresh")
	}
}

func TestExtractRefreshToken_FallbackToBearer(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer bearer-refresh")

	got := extractRefreshToken(r)
	if got != "bearer-refresh" {
		t.Errorf("got %q, want %q", got, "bearer-refresh")
	}
}

func TestExtractRefreshToken_EmptyCookieFallsBack(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: middleware.RefreshCookieName, Value: ""})
	r.Header.Set("Authorization", "Bearer fallback")

	got := extractRefreshToken(r)
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestLogout_MissingToken(t *testing.T) {
	handler := &Handler{}
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "missing refresh token" {
		t.Errorf("error = %q, want %q", body["error"], "missing refresh token")
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	handler := &Handler{}
	req := httptest.NewRequest("POST", "/auth/refresh", nil)
	w := httptest.NewRecorder()

	handler.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestLogin_BadRequest(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"invalid json", "not json", http.StatusBadRequest},
		{"missing password", `{"email":"test@test.com"}`, http.StatusBadRequest},
		{"missing email", `{"password":"secret"}`, http.StatusBadRequest},
		{"empty body", `{}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			handler.Login(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestLogin_EmailTooLong(t *testing.T) {
	handler := &Handler{}
	longEmail := make([]byte, 300)
	for i := range longEmail {
		longEmail[i] = 'a'
	}
	body := `{"email":"` + string(longEmail) + `@test.com","password":"password123"}`

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLogin_PasswordTooLong(t *testing.T) {
	handler := &Handler{}
	longPass := make([]byte, 80)
	for i := range longPass {
		longPass[i] = 'a'
	}
	body := `{"email":"test@test.com","password":"` + string(longPass) + `"}`

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"key": "value"})

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body[key] = %q, want %q", body["key"], "value")
	}
}
