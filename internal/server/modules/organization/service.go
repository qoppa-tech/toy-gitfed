package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
)

var ErrNotFound = errors.New("organization not found")

type Service struct {
	queries *sqlc.Queries
}

func NewService(queries *sqlc.Queries) *Service {
	return &Service{queries: queries}
}

type CreateInput struct {
	Name        string
	Description string
}

func (s *Service) Create(ctx context.Context, input CreateInput) (sqlc.Organization, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return sqlc.Organization{}, fmt.Errorf("generate uuid: %w", err)
	}

	org, err := s.queries.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		OrganizationID:          pgtype.UUID{Bytes: id, Valid: true},
		OrganizationName:        input.Name,
		OrganizationDescription: pgtype.Text{String: input.Description, Valid: input.Description != ""},
	})
	if err != nil {
		return sqlc.Organization{}, fmt.Errorf("create org: %w", err)
	}
	return org, nil
}

func (s *Service) GetByID(ctx context.Context, id pgtype.UUID) (sqlc.Organization, error) {
	org, err := s.queries.GetOrganizationByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return sqlc.Organization{}, ErrNotFound
	}
	return org, err
}

func (s *Service) AddUser(ctx context.Context, orgID, userID pgtype.UUID) error {
	return s.queries.AddUserToOrganization(ctx, sqlc.AddUserToOrganizationParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
}

func (s *Service) RemoveUser(ctx context.Context, orgID, userID pgtype.UUID) error {
	return s.queries.RemoveUserFromOrganization(ctx, sqlc.RemoveUserFromOrganizationParams{
		OrganizationID: orgID,
		UserID:         userID,
	})
}

func (s *Service) GetByUserID(ctx context.Context, userID pgtype.UUID) ([]sqlc.Organization, error) {
	return s.queries.GetOrganizationsByUserID(ctx, userID)
}
