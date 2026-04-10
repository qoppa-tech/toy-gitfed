package user

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestVerifyPassword(t *testing.T) {
	svc := &Service{}

	hashed, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	tests := []struct {
		name    string
		hashed  string
		plain   string
		wantErr bool
	}{
		{
			name:    "correct password",
			hashed:  string(hashed),
			plain:   "correct-password",
			wantErr: false,
		},
		{
			name:    "wrong password",
			hashed:  string(hashed),
			plain:   "wrong-password",
			wantErr: true,
		},
		{
			name:    "empty password",
			hashed:  string(hashed),
			plain:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.VerifyPassword(tt.hashed, tt.plain)
			if (err != nil) != tt.wantErr {
				t.Errorf("VerifyPassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
