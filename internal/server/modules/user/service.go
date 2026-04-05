package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
)

var (
	ErrEmailTaken    = errors.New("email already taken")
	ErrUsernameTaken = errors.New("username already taken")
	ErrNotFound      = errors.New("user not found")
)

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{queries: queries}
}

type RegisterInput struct {
	Name     string
	Username string
	Password string
	Email    string
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (sqlc.User, error) {
	// Check email uniqueness.
	_, err := s.queries.GetUserByEmail(ctx, input.Email)
	if err == nil {
		return sqlc.User{}, ErrEmailTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, fmt.Errorf("check email: %w", err)
	}

	// Check username uniqueness.
	_, err = s.queries.GetUserByUsername(ctx, input.Username)
	if err == nil {
		return sqlc.User{}, ErrUsernameTaken
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, fmt.Errorf("check username: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return sqlc.User{}, fmt.Errorf("hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return sqlc.User{}, fmt.Errorf("generate uuid: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, sqlc.CreateUserParams{
		UserID:   pgtype.UUID{Bytes: id, Valid: true},
		Name:     input.Name,
		Username: input.Username,
		Password: string(hashed),
		Email:    input.Email,
	})
	if err != nil {
		return sqlc.User{}, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *Service) GetByID(ctx context.Context, id pgtype.UUID) (sqlc.User, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, ErrNotFound
	}
	return user, err
}

func (s *Service) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	user, err := s.queries.GetUserByEmail(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.User{}, ErrNotFound
	}
	return user, err
}

func (s *Service) VerifyPassword(hashed, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
}
