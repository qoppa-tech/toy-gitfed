package git

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/google/uuid"
)

var validRepoName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

func ValidateRepoName(name string) error {
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidRepoName, name)
	}
	return nil
}

func SanitizeRepoPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "..", "")
	path = strings.Trim(path, "/")
	return path
}

func BuildRepoPath(base string, components ...string) string {
	parts := []string{base}
	for _, c := range components {
		parts = append(parts, SanitizeRepoPath(c))
	}
	return strings.Join(parts, "/")
}

// Service orchestrates repository management and Smart HTTP pack operations.
type Service struct {
	reposDir string
}

// NewService creates a new Git service rooted at reposDir.
func NewService(reposDir string) *Service {
	return &Service{reposDir: reposDir}
}

func (s *Service) openStorer(repo GitRepository) (*filesystem.Storage, error) {
	path := s.RepoPath(repo)
	dot := osfs.New(path, osfs.WithBoundOS())
	return filesystem.NewStorageWithOptions(dot, cache.NewObjectLRUDefault(), filesystem.Options{}), nil
}

// --- RepositoryStore implementation ---

func (s *Service) Create(ctx context.Context, input CreateInput) (GitRepository, error) {
	if !validRepoName.MatchString(input.Name) {
		return GitRepository{}, fmt.Errorf("%w: %q", ErrInvalidRepoName, input.Name)
	}

	repo := GitRepository{
		ID:          uuid.New(),
		Name:        input.Name,
		Description: input.Description,
		IsPrivate:   input.IsPrivate,
		OwnerID:     input.OwnerID,
		DefaultRef:  input.DefaultRef,
	}

	path := s.RepoPath(repo)
	opts := []git.InitOption{}
	if repo.DefaultRef != "" {
		opts = append(opts, git.WithDefaultBranch(plumbing.ReferenceName(repo.DefaultRef)))
	}

	_, err := git.PlainInit(path, true, opts...)
	if err != nil {
		return GitRepository{}, fmt.Errorf("init bare repo: %w", err)
	}

	return repo, nil
}

func (s *Service) GetByID(_ context.Context, id uuid.UUID) (GitRepository, error) {
	return GitRepository{}, fmt.Errorf("%w: lookup by ID not implemented", ErrRepoNotFound)
}

func (s *Service) GetByName(_ context.Context, ownerID uuid.UUID, name string) (GitRepository, error) {
	repo := GitRepository{
		ID:      uuid.New(),
		Name:    name,
		OwnerID: ownerID,
	}
	path := s.RepoPath(repo)
	fs := osfs.New(path)
	if _, err := fs.Stat("config"); err != nil {
		return GitRepository{}, fmt.Errorf("%w: %s", ErrRepoNotFound, name)
	}
	return repo, nil
}

func (s *Service) ListByOwner(_ context.Context, ownerID uuid.UUID) ([]Repository, error) {
	return nil, fmt.Errorf("list by owner: not implemented without database")
}

func (s *Service) Update(_ context.Context, id uuid.UUID, input UpdateInput) (GitRepository, error) {
	return GitRepository{}, fmt.Errorf("update: not implemented without database")
}

func (s *Service) Delete(_ context.Context, id uuid.UUID) error {
	return fmt.Errorf("delete: not implemented without database")
}

func (s *Service) GetRefs(_ context.Context, repo GitRepository) ([]RefInfo, error) {
	storer, err := s.openStorer(repo)
	if err != nil {
		return nil, fmt.Errorf("open storer: %w", err)
	}
	defer storer.Close()

	iter, err := storer.IterReferences()
	if err != nil {
		return nil, fmt.Errorf("iter references: %w", err)
	}

	var refs []RefInfo
	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		info := RefInfo{
			Name: ref.Name().String(),
			Hash: ref.Hash().String(),
		}
		if target := ref.Target(); target != "" {
			info.Peeled = target.String()
		}
		refs = append(refs, info)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk references: %w", err)
	}

	return refs, nil
}

func (s *Service) GetStats(_ context.Context, repo GitRepository) (RepoStats, error) {
	storer, err := s.openStorer(repo)
	if err != nil {
		return RepoStats{}, fmt.Errorf("open storer: %w", err)
	}
	defer storer.Close()

	var stats RepoStats

	refs, err := storer.IterReferences()
	if err == nil {
		refs.ForEach(func(ref *plumbing.Reference) error {
			if ref.Name().IsBranch() {
				stats.BranchCount++
			}
			if ref.Name().IsTag() {
				stats.TagCount++
			}
			return nil
		})
	}

	commits, err := storer.IterEncodedObjects(plumbing.CommitObject)
	if err == nil {
		commits.ForEach(func(plumbing.EncodedObject) error {
			stats.CommitCount++
			return nil
		})
	}

	head, err := storer.Reference(plumbing.HEAD)
	if err == nil && head != nil {
		commitObj, err := object.GetCommit(storer, head.Hash())
		if err == nil {
			stats.LastCommitTime = commitObj.Committer.When
		}
	}

	return stats, nil
}

func (s *Service) Exists(repo GitRepository) bool {
	path := s.RepoPath(repo)
	fs := osfs.New(path)
	_, err := fs.Stat("config")
	return err == nil
}

func (s *Service) RepoPath(repo GitRepository) string {
	return filepath.Join(s.reposDir, SanitizeRepoPath(repo.Name))
}

// --- PackService implementation ---

func (s *Service) UploadPack(ctx context.Context, req UploadPackRequest, w io.Writer, r io.Reader) error {
	repo := GitRepository{Name: repoNameFromPath(req.RepoPath)}
	storer, err := s.openStorer(repo)
	if err != nil {
		return fmt.Errorf("open storer: %w", err)
	}
	defer storer.Close()

	rc, ok := r.(io.ReadCloser)
	if !ok {
		rc = io.NopCloser(r)
	}
	wc, ok := w.(io.WriteCloser)
	if !ok {
		wc = &nopWriteCloser{w}
	}

	opts := &transport.UploadPackOptions{
		StatelessRPC:  req.StatelessRPC,
		AdvertiseRefs: req.Adverts,
	}

	return transport.UploadPack(ctx, storer, rc, wc, opts)
}

func (s *Service) ReceivePack(ctx context.Context, req ReceivePackRequest, w io.Writer, r io.Reader) error {
	repo := GitRepository{Name: repoNameFromPath(req.RepoPath)}
	storer, err := s.openStorer(repo)
	if err != nil {
		return fmt.Errorf("open storer: %w", err)
	}
	defer storer.Close()

	rc, ok := r.(io.ReadCloser)
	if !ok {
		rc = io.NopCloser(r)
	}
	wc, ok := w.(io.WriteCloser)
	if !ok {
		wc = &nopWriteCloser{w}
	}

	opts := &transport.ReceivePackOptions{
		StatelessRPC: req.StatelessRPC,
	}

	return transport.ReceivePack(ctx, storer, rc, wc, opts)
}

func repoNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

type nopWriteCloser struct {
	io.Writer
}

func (n *nopWriteCloser) Close() error { return nil }
