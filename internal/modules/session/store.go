package session

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/qoppa-tech/gitfed/internal/database/sqlc"
)

var _ Repository = (*PGStore)(nil)

// PGStore implements Repository using sqlc/PostgreSQL.
type PGStore struct {
	q *sqlc.Queries
}

func NewPGStore(q *sqlc.Queries) *PGStore {
	return &PGStore{q: q}
}

func (s *PGStore) Create(ctx context.Context, id, userID uuid.UUID, refreshToken string) error {
	_, err := s.q.CreateSession(ctx, sqlc.CreateSessionParams{
		SessionID:    toPgUUID(id),
		UserID:       toPgUUID(userID),
		RefreshToken: refreshToken,
	})
	return err
}

func (s *PGStore) GetUserIDByRefreshToken(ctx context.Context, refreshToken string) (uuid.UUID, error) {
	row, err := s.q.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrSessionNotFound
		}
		return uuid.Nil, err
	}
	return fromPgUUID(row.UserID), nil
}

func (s *PGStore) DeleteByRefreshToken(ctx context.Context, refreshToken string) error {
	row, err := s.q.GetSessionByRefreshToken(ctx, refreshToken)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrSessionNotFound
		}
		return err
	}
	return s.q.DeleteSession(ctx, row.SessionID)
}

func (s *PGStore) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return s.q.DeleteSessionsByUserID(ctx, toPgUUID(userID))
}

var _ TokenStore = (*RedisTokenStore)(nil)

// RedisTokenStore implements TokenStore using Redis.
type RedisTokenStore struct {
	store redisClient
}

// redisClient is the subset of store.RedisStore methods we need.
type redisClient interface {
	SetAccessToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetAccessToken(ctx context.Context, token string) (string, error)
	SetRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error
	GetRefreshToken(ctx context.Context, token string) (string, error)
	DeleteRefreshToken(ctx context.Context, token string) error
}

func NewRedisTokenStore(rc redisClient) *RedisTokenStore {
	return &RedisTokenStore{store: rc}
}

func (s *RedisTokenStore) SetAccessToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.store.SetAccessToken(ctx, token, userID, ttl)
}

func (s *RedisTokenStore) GetAccessToken(ctx context.Context, token string) (string, error) {
	return s.store.GetAccessToken(ctx, token)
}

func (s *RedisTokenStore) SetRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.store.SetRefreshToken(ctx, token, userID, ttl)
}

func (s *RedisTokenStore) GetRefreshToken(ctx context.Context, token string) (string, error) {
	return s.store.GetRefreshToken(ctx, token)
}

func (s *RedisTokenStore) DeleteRefreshToken(ctx context.Context, token string) error {
	return s.store.DeleteRefreshToken(ctx, token)
}

// --- conversion helpers ---

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func fromPgUUID(u pgtype.UUID) uuid.UUID {
	if !u.Valid {
		return uuid.Nil
	}
	return uuid.UUID(u.Bytes)
}
