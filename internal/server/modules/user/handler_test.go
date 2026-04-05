package user

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestUUIDString(t *testing.T) {
	tests := []struct {
		name string
		uuid pgtype.UUID
		want string
	}{
		{
			name: "valid uuid",
			uuid: pgtype.UUID{
				Bytes: [16]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
					0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
				Valid: true,
			},
			want: "01020304-0506-0708-090a-0b0c0d0e0f10",
		},
		{
			name: "invalid uuid",
			uuid: pgtype.UUID{Valid: false},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uuidString(tt.uuid)
			if got != tt.want {
				t.Errorf("uuidString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})

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

func TestRegisterHandler_BadRequest(t *testing.T) {
	handler := NewHandler(&Service{})

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

			handler.Register(w, req)

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
