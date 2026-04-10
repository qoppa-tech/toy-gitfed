package session

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 7 * 24 * time.Hour
)

var (
	ErrInvalidAccessToken  = errors.New("invalid or expired access token")
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
	ErrSessionNotFound     = errors.New("session not found")
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type Session struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	RefreshToken string
	CreatedAt    time.Time
}
