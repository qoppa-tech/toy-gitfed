package sso

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
)

var _ Repository = (*PGStore)(nil)

// PGStore implements Repository using sqlc/PostgreSQL.
type PGStore struct {
	q *sqlc.Queries
}

func NewPGStore(q *sqlc.Queries) *PGStore {
	return &PGStore{q: q}
}

func (s *PGStore) Create(ctx context.Context, r Record) (Record, error) {
	row, err := s.q.CreateSSO(ctx, sqlc.CreateSSOParams{
		SsoID:    toPgUUID(r.ID),
		UserID:   toPgUUID(r.UserID),
		Name:     r.Name,
		Provider: string(r.Provider),
		Username: pgtype.Text{String: r.Username, Valid: r.Username != ""},
	})
	if err != nil {
		return Record{}, err
	}
	return fromSqlcSSO(row), nil
}

func (s *PGStore) GetByProviderAndUsername(ctx context.Context, provider Provider, username string) (Record, error) {
	row, err := s.q.GetSSOByProviderAndUsername(ctx, sqlc.GetSSOByProviderAndUsernameParams{
		Provider: string(provider),
		Username: pgtype.Text{String: username, Valid: username != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Record{}, ErrProviderNotFound
		}
		return Record{}, err
	}
	return fromSqlcSSO(row), nil
}

func (s *PGStore) GetByUserID(ctx context.Context, userID uuid.UUID) ([]Record, error) {
	rows, err := s.q.GetSSOByUserID(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}
	records := make([]Record, len(rows))
	for i, row := range rows {
		records[i] = fromSqlcSSO(row)
	}
	return records, nil
}

func (s *PGStore) Delete(ctx context.Context, id uuid.UUID) error {
	return s.q.DeleteSSO(ctx, toPgUUID(id))
}

var _ StateStore = (*RedisStateStore)(nil)

// RedisStateStore implements StateStore using Redis.
type RedisStateStore struct {
	store oauthStateClient
}

// oauthStateClient is the subset of store.RedisStore methods we need.
type oauthStateClient interface {
	SetOAuthState(ctx context.Context, state string, ttl time.Duration) error
	GetOAuthState(ctx context.Context, state string) (bool, error)
	DeleteOAuthState(ctx context.Context, state string) error
}

func NewRedisStateStore(rc oauthStateClient) *RedisStateStore {
	return &RedisStateStore{store: rc}
}

func (s *RedisStateStore) SetOAuthState(ctx context.Context, state string, ttl time.Duration) error {
	return s.store.SetOAuthState(ctx, state, ttl)
}

func (s *RedisStateStore) GetOAuthState(ctx context.Context, state string) (bool, error) {
	return s.store.GetOAuthState(ctx, state)
}

func (s *RedisStateStore) DeleteOAuthState(ctx context.Context, state string) error {
	return s.store.DeleteOAuthState(ctx, state)
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

func fromSqlcSSO(s sqlc.SsoProvider) Record {
	return Record{
		ID:          fromPgUUID(s.SsoID),
		UserID:      fromPgUUID(s.UserID),
		Name:        s.Name,
		Provider:    Provider(s.Provider),
		Username:    s.Username.String,
		ActivatedAt: s.ActivatedAt.Time,
		CreatedAt:   s.CreatedAt.Time,
	}
}
