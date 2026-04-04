// Package git implements repository management and Smart HTTP protocol interfaces.
//
// It provides:
//   - Repository lifecycle management (create, delete, inspect)
//   - Upload-pack (fetch/clone) and receive-pack (push) operations
//   - Integration with go-git's Smart HTTP backend
package git

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRepoNotFound      = errors.New("repository not found")
	ErrRepoAlreadyExists = errors.New("repository already exists")
	ErrInvalidRepoName   = errors.New("invalid repository name")
	ErrRepoAccessDenied  = errors.New("repository access denied")
)

// Repository represents a managed Git repository.
type Repository struct {
	ID          uuid.UUID
	Name        string
	Description string
	IsPrivate   bool
	IsDeleted   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	OwnerID     uuid.UUID
	DefaultRef  string
	Head        string
}

// CreateInput holds the parameters for creating a new repository.
type CreateInput struct {
	Name        string
	Description string
	IsPrivate   bool
	OwnerID     uuid.UUID
	DefaultRef  string
}

// UpdateInput holds the parameters for updating a repository.
type UpdateInput struct {
	Description *string
	IsPrivate   *bool
	DefaultRef  *string
}

// RefInfo describes a Git reference in a repository.
type RefInfo struct {
	Name   string
	Hash   string
	Peeled string
}

// RepoStats holds statistics about a repository.
type RepoStats struct {
	BranchCount    int
	TagCount       int
	CommitCount    int64
	SizeBytes      uint64
	LastCommitTime time.Time
}

// UploadPackRequest represents a request to fetch objects from a repository.
type UploadPackRequest struct {
	RepoPath     string
	Adverts      bool
	StatelessRPC bool
}

// ReceivePackRequest represents a request to push objects to a repository.
type ReceivePackRequest struct {
	RepoPath     string
	StatelessRPC bool
}
