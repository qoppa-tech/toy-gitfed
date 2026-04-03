package session

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository is the persistent store for sessions (PostgreSQL).
type Repository interface {
	Create(ctx context.Context, id, userID uuid.UUID, refreshToken string) error
	GetUserIDByRefreshToken(ctx context.Context, refreshToken string) (uuid.UUID, error)
	DeleteByRefreshToken(ctx context.Context, refreshToken string) error
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
}

// TokenStore is the fast token cache (Redis).
type TokenStore interface {
	SetAccessToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetAccessToken(ctx context.Context, token string) (string, error)
	SetRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, token string) (string, error)
	DeleteRefreshToken(ctx context.Context, token string) error
}
