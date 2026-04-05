package sso

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestPgUUIDString(t *testing.T) {
	tests := []struct {
		name string
		uuid pgtype.UUID
		want string
	}{
		{
			name: "valid uuid",
			uuid: pgtype.UUID{
				Bytes: [16]byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11,
					0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99},
				Valid: true,
			},
			want: "aabbccdd-eeff-0011-2233-445566778899",
		},
		{
			name: "invalid uuid",
			uuid: pgtype.UUID{Valid: false},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pgUUIDString(tt.uuid)
			if got != tt.want {
				t.Errorf("pgUUIDString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"test": "value"})

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
	if body["test"] != "value" {
		t.Errorf("body[test] = %q, want %q", body["test"], "value")
	}
}

func TestGoogleCallback_MissingParams(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing both",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing code",
			query:      "state=abc",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing state",
			query:      "code=abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/auth/google/callback?"+tt.query, nil)
			w := httptest.NewRecorder()

			handler.GoogleCallback(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
