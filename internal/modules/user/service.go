package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (User, error) {
	_, err := s.repo.GetByEmail(ctx, input.Email)
	if err == nil {
		return User{}, ErrEmailTaken
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, fmt.Errorf("check email: %w", err)
	}

	_, err = s.repo.GetByUsername(ctx, input.Username)
	if err == nil {
		return User{}, ErrUsernameTaken
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, fmt.Errorf("check username: %w", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("generate uuid: %w", err)
	}

	u, err := s.repo.Create(ctx, User{
		ID:       id,
		Name:     input.Name,
		Username: input.Username,
		Password: string(hashed),
		Email:    input.Email,
	})
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByEmail(ctx context.Context, email string) (User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *Service) VerifyPassword(hashed, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
}
