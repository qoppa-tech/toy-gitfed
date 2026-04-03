package organization

import (
	"context"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, org Organization) (Organization, error)
	GetByID(ctx context.Context, id uuid.UUID) (Organization, error)
	AddUser(ctx context.Context, orgID, userID uuid.UUID) error
	RemoveUser(ctx context.Context, orgID, userID uuid.UUID) error
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]Organization, error)
}
