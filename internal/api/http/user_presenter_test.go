package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterHandler_BadRequest(t *testing.T) {
	presenter := NewUserPresenter(nil)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing fields",
			body:       `{"name":"test"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			body:       `{}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/auth/register", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			presenter.Register(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestValidateRegisterInput(t *testing.T) {
	valid := registerRequest{
		Name:     "Test User",
		Username: "testuser",
		Password: "securepassword",
		Email:    "test@example.com",
	}

	tests := []struct {
		name    string
		modify  func(r registerRequest) registerRequest
		wantErr string
	}{
		{
			name:    "valid input",
			modify:  func(r registerRequest) registerRequest { return r },
			wantErr: "",
		},
		{
			name: "name too long",
			modify: func(r registerRequest) registerRequest {
				r.Name = strings.Repeat("a", 256)
				return r
			},
			wantErr: "name too long",
		},
		{
			name: "username too long",
			modify: func(r registerRequest) registerRequest {
				r.Username = strings.Repeat("a", 256)
				return r
			},
			wantErr: "username too long",
		},
		{
			name: "email too long",
			modify: func(r registerRequest) registerRequest {
				r.Email = strings.Repeat("a", 250) + "@b.com"
				return r
			},
			wantErr: "email too long",
		},
		{
			name: "invalid email format",
			modify: func(r registerRequest) registerRequest {
				r.Email = "not-an-email"
				return r
			},
			wantErr: "invalid email format",
		},
		{
			name: "password too short",
			modify: func(r registerRequest) registerRequest {
				r.Password = "short"
				return r
			},
			wantErr: "password must be at least 8 characters",
		},
		{
			name: "password too long (bcrypt limit)",
			modify: func(r registerRequest) registerRequest {
				r.Password = strings.Repeat("a", 73)
				return r
			},
			wantErr: "password too long",
		},
		{
			name: "email without TLD",
			modify: func(r registerRequest) registerRequest {
				r.Email = "user@localhost"
				return r
			},
			wantErr: "invalid email format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.modify(valid)
			err := validateRegisterInput(input)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error %q, got nil", tt.wantErr)
				return
			}
			if err.Error() != tt.wantErr {
				t.Errorf("error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestEmailRegex(t *testing.T) {
	valid := []string{
		"user@example.com",
		"user.name@example.co.uk",
		"user+tag@example.com",
		"user-name@example.org",
		"user_name@example.io",
		"u@ex.co",
	}
	invalid := []string{
		"",
		"@example.com",
		"user@",
		"user@.com",
		"user@com",
		"user example.com",
		"user@@example.com",
	}

	for _, email := range valid {
		if !emailRegex.MatchString(email) {
			t.Errorf("email %q should be valid", email)
		}
	}
	for _, email := range invalid {
		if emailRegex.MatchString(email) {
			t.Errorf("email %q should be invalid", email)
		}
	}
}
