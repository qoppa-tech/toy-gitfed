package session

import (
	"testing"
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
