package sso

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistent store for SSO records (PostgreSQL).
type Repository interface {
	Create(ctx context.Context, r Record) (Record, error)
	GetByProviderAndUsername(ctx context.Context, provider Provider, username string) (Record, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Record, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

// StateStore manages OAuth state tokens (Redis).
type StateStore interface {
	SetOAuthState(ctx context.Context, state string, ttl time.Duration) error
	GetOAuthState(ctx context.Context, state string) (bool, error)
	DeleteOAuthState(ctx context.Context, state string) error
}
