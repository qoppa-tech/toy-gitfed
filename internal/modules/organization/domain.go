package organization

import (
	"errors"

	"github.com/google/uuid"
)

var ErrNotFound = errors.New("organization not found")

type Organization struct {
	ID          uuid.UUID
	Name        string
	Description string
}

type CreateInput struct {
	Name        string
	Description string
}
