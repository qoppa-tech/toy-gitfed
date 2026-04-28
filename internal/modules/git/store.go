package git

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/qoppa-tech/gitfed/internal/database/sqlc"
)

var _ Repository = (*Store)(nil)

type Store struct {
	q *sqlc.Queries
}

func NewStore(q *sqlc.Queries) *Store {
	return &Store{q: q}
}

func (s *Store) Create(ctx context.Context, input CreateInput) (GitRepository, error) {
	row, err := s.q.CreateRepository(ctx, sqlc.CreateRepositoryParams{
		ID:          toPgUUID(input.Id),
		Name:        input.Name,
		Description: pgtype.Text{String: input.Description, Valid: input.Description != ""},
		IsPrivate:   input.IsPrivate,
		OwnerID:     toPgUUID(input.OwnerID),
		DefaultRef:  input.DefaultRef,
	})
	if err != nil {
		return GitRepository{}, err
	}
	return fromSqlcRepo(row), nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (GitRepository, error) {
	row, err := s.q.GetRepositoryByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GitRepository{}, ErrRepoNotFound
		}
		return GitRepository{}, err
	}
	return fromSqlcRepo(row), nil
}

func (s *Store) GetByName(ctx context.Context, ownerID uuid.UUID, name string) (GitRepository, error) {
	rows, err := s.q.GetRepositoryByRepositoryName(ctx, name)
	if err != nil {
		return GitRepository{}, err
	}
	for _, row := range rows {
		if row.OwnerID == toPgUUID(ownerID) && !row.IsDeleted {
			return fromSqlcRepo(row), nil
		}
	}
	return GitRepository{}, ErrRepoNotFound
}

func (s *Store) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]GitRepository, error) {
	rows, err := s.q.GetRepositoryByOwnerId(ctx, toPgUUID(ownerID))
	if err != nil {
		return nil, err
	}
	repos := make([]GitRepository, 0, len(rows))
	for _, row := range rows {
		if !row.IsDeleted {
			repos = append(repos, fromSqlcRepo(row))
		}
	}
	return repos, nil
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, input UpdateInput) (GitRepository, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return GitRepository{}, err
	}

	name := existing.Name
	if input.Name != nil {
		name = *input.Name
	}

	description := existing.Description
	if input.Description != nil {
		description = *input.Description
	}

	isPrivate := existing.IsPrivate
	if input.IsPrivate != nil {
		isPrivate = *input.IsPrivate
	}

	ownerID := existing.OwnerID
	if input.OwnerId != nil {
		ownerID = *input.OwnerId
	}

	defaultRef := existing.DefaultRef
	if input.DefaultRef != nil {
		defaultRef = *input.DefaultRef
	}

	err = s.q.UpdateRepository(ctx, sqlc.UpdateRepositoryParams{
		ID:          toPgUUID(id),
		Name:        name,
		Description: pgtype.Text{String: description, Valid: description != ""},
		IsPrivate:   isPrivate,
		OwnerID:     toPgUUID(ownerID),
		DefaultRef:  defaultRef,
	})
	if err != nil {
		return GitRepository{}, err
	}

	return s.GetByID(ctx, id)
}

func (s *Store) SoftDelete(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.SoftDeleteRepository(ctx, toPgUUID(id))
	return err
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	return s.SoftDelete(ctx, id)
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

func fromSqlcRepo(g sqlc.GitRepository) GitRepository {
	return GitRepository{
		ID:          fromPgUUID(g.ID),
		Name:        g.Name,
		Description: g.Description.String,
		IsPrivate:   g.IsPrivate,
		IsDeleted:   g.IsDeleted,
		CreatedAt:   g.CreatedAt.Time,
		UpdatedAt:   g.UpdatedAt.Time,
		OwnerID:     fromPgUUID(g.OwnerID),
		DefaultRef:  g.DefaultRef,
		Head:        g.Head,
	}
}
