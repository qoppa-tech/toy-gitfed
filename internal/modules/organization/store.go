package organization

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

func (s *Store) Create(ctx context.Context, org Organization) (Organization, error) {
	row, err := s.q.CreateOrganization(ctx, sqlc.CreateOrganizationParams{
		OrganizationID:          toPgUUID(org.ID),
		OrganizationName:        org.Name,
		OrganizationDescription: pgtype.Text{String: org.Description, Valid: org.Description != ""},
	})
	if err != nil {
		return Organization{}, err
	}
	return fromSqlcOrg(row), nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (Organization, error) {
	row, err := s.q.GetOrganizationByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Organization{}, ErrNotFound
		}
		return Organization{}, err
	}
	return fromSqlcOrg(row), nil
}

func (s *Store) AddUser(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.q.AddUserToOrganization(ctx, sqlc.AddUserToOrganizationParams{
		OrganizationID: toPgUUID(orgID),
		UserID:         toPgUUID(userID),
	})
}

func (s *Store) RemoveUser(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.q.RemoveUserFromOrganization(ctx, sqlc.RemoveUserFromOrganizationParams{
		OrganizationID: toPgUUID(orgID),
		UserID:         toPgUUID(userID),
	})
}

func (s *Store) GetByUserID(ctx context.Context, userID uuid.UUID) ([]Organization, error) {
	rows, err := s.q.GetOrganizationsByUserID(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}
	orgs := make([]Organization, len(rows))
	for i, row := range rows {
		orgs[i] = fromSqlcOrg(row)
	}
	return orgs, nil
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

func fromSqlcOrg(o sqlc.Organization) Organization {
	return Organization{
		ID:          fromPgUUID(o.OrganizationID),
		Name:        o.OrganizationName,
		Description: o.OrganizationDescription.String,
	}
}
