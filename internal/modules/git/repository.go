package git

import (
	"context"
	"io"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, input CreateInput) (GitRepository, error)
	GetByID(ctx context.Context, id uuid.UUID) (GitRepository, error)
	GetByName(ctx context.Context, ownerID uuid.UUID, name string) (GitRepository, error)
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]GitRepository, error)
	Update(ctx context.Context, id uuid.UUID, input UpdateInput) (GitRepository, error)
	SoftDelete(ctx context.Context, id uuid.UUID) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// PackService defines the interface for Smart HTTP pack operations.
type PackService interface {
	// UploadPack serves the advertisement of refs and capabilities, or processes
	// a fetch request and writes the packfile to the response writer.
	UploadPack(ctx context.Context, req UploadPackRequest, w io.Writer, r io.Reader) error

	// ReceivePack processes a push request, reads the incoming packfile from the
	// reader, and writes the server response to the writer.
	ReceivePack(ctx context.Context, req ReceivePackRequest, w io.Writer, r io.Reader) error
}
