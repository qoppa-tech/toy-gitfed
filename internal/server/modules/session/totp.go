package session

import (
	"context"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/qoppa-tech/toy-gitfed/internal/store"
)

const totpTTL = 5 * time.Minute

type TOTPService struct {
	redis  *store.RedisStore
	issuer string
}

func NewTOTPService(redis *store.RedisStore, issuer string) *TOTPService {
	return &TOTPService{redis: redis, issuer: issuer}
}

type TOTPSetupResult struct {
	Secret string `json:"secret"`
	URL    string `json:"url"`
}

// Setup generates a new TOTP key and stores the secret in Redis for verification.
func (t *TOTPService) Setup(ctx context.Context, userID, accountName string) (*TOTPSetupResult, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      t.issuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp: %w", err)
	}

	if err := t.redis.SetTOTPSecret(ctx, userID, key.Secret(), totpTTL); err != nil {
		return nil, fmt.Errorf("store totp secret: %w", err)
	}

	return &TOTPSetupResult{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

// Verify validates a TOTP code against the secret stored in Redis.
func (t *TOTPService) Verify(ctx context.Context, userID, code string) (bool, error) {
	secret, err := t.redis.GetTOTPSecret(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get totp secret: %w", err)
	}

	valid := totp.Validate(code, secret)
	if valid {
		_ = t.redis.DeleteTOTPSecret(ctx, userID)
	}
	return valid, nil
}
