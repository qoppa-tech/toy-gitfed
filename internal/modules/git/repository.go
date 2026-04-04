package git

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// RepositoryManager defines the interface for Git repository operations.
// Implementations handle the filesystem and database persistence layer.
type RepositoryManager interface {
	// Create initializes a new bare Git repository on disk and persists metadata.
	Create(ctx context.Context, input CreateInput) (Repository, error)

	// GetByID retrieves repository metadata by ID.
	GetByID(ctx context.Context, id uuid.UUID) (Repository, error)

	// GetByName retrieves repository metadata by owner and name.
	GetByName(ctx context.Context, ownerID uuid.UUID, name string) (Repository, error)

	// ListByOwner returns all repositories owned by a user/organization.
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]Repository, error)

	// Update modifies mutable fields of an existing repository.
	Update(ctx context.Context, id uuid.UUID, input UpdateInput) (Repository, error)

	// Delete soft-deletes a repository and removes it from disk.
	Delete(ctx context.Context, id uuid.UUID) error

	// GetRefs returns all references in a repository.
	GetRefs(ctx context.Context, repo Repository) ([]RefInfo, error)

	// GetStats computes repository statistics.
	GetStats(ctx context.Context, repo Repository) (RepoStats, error)

	// Exists checks if a repository directory exists on disk.
	Exists(repo Repository) bool

	// RepoPath returns the absolute filesystem path for a repository.
	RepoPath(repo Repository) string
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
