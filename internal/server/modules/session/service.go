package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
	"github.com/qoppa-tech/toy-gitfed/internal/store"
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

// TokenPair holds the access and refresh tokens returned on login/refresh.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type Service struct {
	queries *sqlc.Queries
	redis   *store.RedisStore
}

func NewService(queries *sqlc.Queries, redis *store.RedisStore) *Service {
	return &Service{
		queries: queries,
		redis:   redis,
	}
}

// Create generates a new access + refresh token pair.
// Refresh token is persisted in PostgreSQL and Redis; access token in Redis only.
func (s *Service) Create(ctx context.Context, userID pgtype.UUID) (TokenPair, error) {
	accessToken, err := generateToken(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := generateToken(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	sessionID, err := uuid.NewV7()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate uuid: %w", err)
	}

	// Persist refresh token in PostgreSQL.
	if _, err := s.queries.CreateSession(ctx, sqlc.CreateSessionParams{
		SessionID:    pgtype.UUID{Bytes: sessionID, Valid: true},
		UserID:       userID,
		RefreshToken: refreshToken,
	}); err != nil {
		return TokenPair{}, fmt.Errorf("create session: %w", err)
	}

	uidStr := uuidToString(userID)

	// Store access token in Redis (short-lived).
	if err := s.redis.SetAccessToken(ctx, accessToken, uidStr, AccessTokenTTL); err != nil {
		return TokenPair{}, fmt.Errorf("redis set access token: %w", err)
	}

	// Store refresh token in Redis (long-lived cache).
	if err := s.redis.SetRefreshToken(ctx, refreshToken, uidStr, RefreshTokenTTL); err != nil {
		return TokenPair{}, fmt.Errorf("redis set refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateAccess checks an access token against Redis.
// This is the hot path used by the auth middleware.
func (s *Service) ValidateAccess(ctx context.Context, accessToken string) (pgtype.UUID, error) {
	uidStr, err := s.redis.GetAccessToken(ctx, accessToken)
	if err != nil || uidStr == "" {
		return pgtype.UUID{}, ErrInvalidAccessToken
	}
	return parseUUID(uidStr)
}

// Validate implements middleware.TokenValidator using access tokens.
func (s *Service) Validate(ctx context.Context, token string) (pgtype.UUID, error) {
	return s.ValidateAccess(ctx, token)
}

// Refresh validates a refresh token and issues a new access token.
// The refresh token itself is not rotated — it stays valid until logout or expiry.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	// Fast path: check Redis.
	uidStr, err := s.redis.GetRefreshToken(ctx, refreshToken)
	if err != nil || uidStr == "" {
		// Slow path: check PostgreSQL.
		sess, err := s.queries.GetSessionByRefreshToken(ctx, refreshToken)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrInvalidRefreshToken
		}
		if err != nil {
			return "", fmt.Errorf("query session: %w", err)
		}
		uidStr = uuidToString(sess.UserID)

		// Re-populate Redis cache.
		_ = s.redis.SetRefreshToken(ctx, refreshToken, uidStr, RefreshTokenTTL)
	}

	// Mint a new access token.
	accessToken, err := generateToken(32)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	if err := s.redis.SetAccessToken(ctx, accessToken, uidStr, AccessTokenTTL); err != nil {
		return "", fmt.Errorf("redis set access token: %w", err)
	}

	return accessToken, nil
}

// Revoke deletes a session and invalidates both tokens.
func (s *Service) Revoke(ctx context.Context, refreshToken string) error {
	sess, err := s.queries.GetSessionByRefreshToken(ctx, refreshToken)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return fmt.Errorf("query session: %w", err)
	}

	if err := s.queries.DeleteSession(ctx, sess.SessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	// Best-effort cleanup of Redis keys.
	_ = s.redis.DeleteRefreshToken(ctx, refreshToken)
	// Access tokens expire naturally (15 min) — no tracking needed.
	return nil
}

// RevokeAll deletes all sessions for a user.
func (s *Service) RevokeAll(ctx context.Context, userID pgtype.UUID) error {
	return s.queries.DeleteSessionsByUserID(ctx, userID)
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func uuidToString(u pgtype.UUID) string {
	if !u.Valid {
		return ""
	}
	id, _ := uuid.FromBytes(u.Bytes[:])
	return id.String()
}

func parseUUID(s string) (pgtype.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{}, err
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}
