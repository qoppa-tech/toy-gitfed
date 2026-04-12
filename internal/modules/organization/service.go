package organization

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Organization, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Organization{}, fmt.Errorf("generate uuid: %w", err)
	}

	org, err := s.repo.Create(ctx, Organization{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		return Organization{}, fmt.Errorf("create org: %w", err)
	}
	return org, nil
}

func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (Organization, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteByID(ctx, id)
}

func (s *Service) AddUser(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.repo.AddUser(ctx, orgID, userID)
}

func (s *Service) RemoveUser(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.repo.RemoveUser(ctx, orgID, userID)
}

func (s *Service) GetByUserID(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	return s.repo.GetByUserID(ctx, userID)
}
