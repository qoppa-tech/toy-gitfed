package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
)

var _ Repository = (*Store)(nil)

type Store struct {
	q *sqlc.Queries
}

func NewStore(q *sqlc.Queries) *Store {
	return &Store{q: q}
}

func (s *Store) Create(ctx context.Context, u User) (User, error) {
	row, err := s.q.CreateUser(ctx, sqlc.CreateUserParams{
		UserID:   toPgUUID(u.ID),
		Name:     u.Name,
		Username: u.Username,
		Password: u.Password,
		Email:    u.Email,
	})
	if err != nil {
		return User{}, err
	}
	return fromSqlcUser(row), nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	row, err := s.q.GetUserByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromSqlcUser(row), nil
}

func (s *Store) GetByEmail(ctx context.Context, email string) (User, error) {
	row, err := s.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromSqlcUser(row), nil
}

func (s *Store) GetByUsername(ctx context.Context, username string) (User, error) {
	row, err := s.q.GetUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, err
	}
	return fromSqlcUser(row), nil
}

func (s *Store) Verify(ctx context.Context, id uuid.UUID) error {
	return s.q.VerifyUser(ctx, toPgUUID(id))
}

func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	return s.q.SoftDeleteUser(ctx, toPgUUID(id))
}

func (s *Store) UpdatePassword(ctx context.Context, id uuid.UUID, password string) error {
	return s.q.UpdateUserPassword(ctx, sqlc.UpdateUserPasswordParams{
		UserID:   toPgUUID(id),
		Password: password,
	})
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

func fromSqlcUser(u sqlc.User) User {
	return User{
		ID:         fromPgUUID(u.UserID),
		Name:       u.Name,
		Username:   u.Username,
		Email:      u.Email,
		Password:   u.Password,
		CreatedAt:  u.CreatedAt.Time,
		UpdatedAt:  u.UpdatedAt.Time,
		IsDeleted:  u.IsDeleted,
		IsVerified: u.IsVerified,
	}
}
