package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractRefreshToken_CookieFirst(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: "cookie-refresh"})
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
	r.AddCookie(&http.Cookie{Name: RefreshCookieName, Value: ""})
	r.Header.Set("Authorization", "Bearer fallback")

	got := extractRefreshToken(r)
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestLogout_MissingToken(t *testing.T) {
	presenter := &SessionPresenter{}
	req := httptest.NewRequest("POST", "/auth/logout", nil)
	w := httptest.NewRecorder()

	presenter.Logout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_MissingToken(t *testing.T) {
	presenter := &SessionPresenter{}
	req := httptest.NewRequest("POST", "/auth/refresh", nil)
	w := httptest.NewRecorder()

	presenter.Refresh(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestLogin_BadRequest(t *testing.T) {
	presenter := &SessionPresenter{}

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
			presenter.Login(w, req)
			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestLogin_EmailTooLong(t *testing.T) {
	presenter := &SessionPresenter{}
	longEmail := make([]byte, 300)
	for i := range longEmail {
		longEmail[i] = 'a'
	}
	body := `{"email":"` + string(longEmail) + `@test.com","password":"password123"}`

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	presenter.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLogin_PasswordTooLong(t *testing.T) {
	presenter := &SessionPresenter{}
	longPass := make([]byte, 80)
	for i := range longPass {
		longPass[i] = 'a'
	}
	body := `{"email":"test@test.com","password":"` + string(longPass) + `"}`

	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	presenter.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
