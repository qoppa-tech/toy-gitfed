package session

import (
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
)

func TestTOTPSetupResult_Fields(t *testing.T) {
	r := TOTPSetupResult{
		Secret: "JBSWY3DPEHPK3PXP",
		URL:    "otpauth://totp/gitfed:user?secret=JBSWY3DPEHPK3PXP&issuer=gitfed",
	}

	if r.Secret == "" {
		t.Error("Secret should not be empty")
	}
	if r.URL == "" {
		t.Error("URL should not be empty")
	}
}

func TestTOTPValidation_Logic(t *testing.T) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "test",
		AccountName: "user@test.com",
	})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	if !totp.Validate(code, key.Secret()) {
		t.Error("valid code should pass validation")
	}

	if totp.Validate("000000", key.Secret()) {
		t.Log("unlikely: 000000 happened to be valid")
	}
}
