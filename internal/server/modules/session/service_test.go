package session

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestGenerateToken(t *testing.T) {
	token1, err := generateToken(32)
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}
	if len(token1) != 64 { // hex encoded 32 bytes = 64 chars
		t.Errorf("token length = %d, want 64", len(token1))
	}

	token2, err := generateToken(32)
	if err != nil {
		t.Fatalf("generateToken: %v", err)
	}

	if token1 == token2 {
		t.Error("two generated tokens should not be equal")
	}
}

func TestUUIDToString(t *testing.T) {
	tests := []struct {
		name string
		uuid pgtype.UUID
		want string
	}{
		{
			name: "valid uuid",
			uuid: pgtype.UUID{
				Bytes: [16]byte{0x01, 0x93, 0x4b, 0x6c, 0x9a, 0xd0, 0x70, 0x00,
					0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01},
				Valid: true,
			},
			want: "01934b6c-9ad0-7000-8000-000000000001",
		},
		{
			name: "invalid uuid returns empty",
			uuid: pgtype.UUID{Valid: false},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uuidToString(tt.uuid)
			if got != tt.want {
				t.Errorf("uuidToString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid uuid",
			input:   "01934b6c-9ad0-7000-8000-000000000001",
			wantErr: false,
		},
		{
			name:    "invalid uuid",
			input:   "not-a-uuid",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseUUID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseUUID() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !result.Valid {
				t.Error("parseUUID() returned invalid UUID for valid input")
			}
		})
	}
}

func TestParseUUID_Roundtrip(t *testing.T) {
	original := "01934b6c-9ad0-7000-8000-000000000001"
	parsed, err := parseUUID(original)
	if err != nil {
		t.Fatalf("parseUUID: %v", err)
	}

	result := uuidToString(parsed)
	if result != original {
		t.Errorf("roundtrip: got %q, want %q", result, original)
	}
}
