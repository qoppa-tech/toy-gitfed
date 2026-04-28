package session

import (
	"context"
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/qoppa-tech/gitfed/pkg/logger"
)

const totpTTL = 5 * time.Minute

// TOTPStore is the subset of Redis operations needed by TOTP.
type TOTPStore interface {
	SetTOTPSecret(ctx context.Context, userID string, secret string, ttl time.Duration) error
	GetTOTPSecret(ctx context.Context, userID string) (string, error)
	DeleteTOTPSecret(ctx context.Context, userID string) error
}

type TOTPService struct {
	store  TOTPStore
	issuer string
}

func NewTOTPService(store TOTPStore, issuer string) *TOTPService {
	return &TOTPService{store: store, issuer: issuer}
}

type TOTPSetupResult struct {
	Secret string
	URL    string
}

func (t *TOTPService) Setup(ctx context.Context, userID, accountName string) (*TOTPSetupResult, error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      t.issuer,
		AccountName: accountName,
	})
	if err != nil {
		return nil, fmt.Errorf("generate totp: %w", err)
	}

	if err := t.store.SetTOTPSecret(ctx, userID, key.Secret(), totpTTL); err != nil {
		return nil, fmt.Errorf("store totp secret: %w", err)
	}

	return &TOTPSetupResult{
		Secret: key.Secret(),
		URL:    key.URL(),
	}, nil
}

func (t *TOTPService) Verify(ctx context.Context, userID, code string) (bool, error) {
	secret, err := t.store.GetTOTPSecret(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("get totp secret: %w", err)
	}

	valid := totp.Validate(code, secret)
	if valid {
		if err := t.store.DeleteTOTPSecret(ctx, userID); err != nil {
			logger.FromContext(ctx).Warn("totp secret delete failed", "step", "totp_cleanup", "user_id", userID, "error", err)
		}
	}
	return valid, nil
}
