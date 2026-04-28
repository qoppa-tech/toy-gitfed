package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/internal/modules/git"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type repoTokenValidator struct {
	userID uuid.UUID
	err    error
}

func (v *repoTokenValidator) Validate(_ context.Context, _ string) (uuid.UUID, error) {
	if v.err != nil {
		return uuid.Nil, v.err
	}
	return v.userID, nil
}

type fakeRepoStore struct {
	repos           map[uuid.UUID]git.GitRepository
	softDeleteErr   error
	softDeleteCalls int
	deleteErr       error
	deleteCalls     int
}

func newFakeRepoStore() *fakeRepoStore {
	return &fakeRepoStore{repos: make(map[uuid.UUID]git.GitRepository)}
}

func (s *fakeRepoStore) Create(_ context.Context, input git.CreateInput) (git.GitRepository, error) {
	for _, repo := range s.repos {
		if !repo.IsDeleted && repo.OwnerID == input.OwnerID && repo.Name == input.Name {
			return git.GitRepository{}, git.ErrRepoAlreadyExists
		}
	}
	repo := git.GitRepository{
		ID:          input.Id,
		Name:        input.Name,
		Description: input.Description,
		IsPrivate:   input.IsPrivate,
		OwnerID:     input.OwnerID,
		DefaultRef:  input.DefaultRef,
	}
	s.repos[repo.ID] = repo
	return repo, nil
}

func (s *fakeRepoStore) GetByID(_ context.Context, id uuid.UUID) (git.GitRepository, error) {
	repo, ok := s.repos[id]
	if !ok || repo.IsDeleted {
		return git.GitRepository{}, git.ErrRepoNotFound
	}
	return repo, nil
}

func (s *fakeRepoStore) GetByName(_ context.Context, ownerID uuid.UUID, name string) (git.GitRepository, error) {
	for _, repo := range s.repos {
		if !repo.IsDeleted && repo.OwnerID == ownerID && repo.Name == name {
			return repo, nil
		}
	}
	return git.GitRepository{}, git.ErrRepoNotFound
}

func (s *fakeRepoStore) ListByOwner(_ context.Context, ownerID uuid.UUID) ([]git.GitRepository, error) {
	repos := make([]git.GitRepository, 0)
	for _, repo := range s.repos {
		if !repo.IsDeleted && repo.OwnerID == ownerID {
			repos = append(repos, repo)
		}
	}
	return repos, nil
}

func (s *fakeRepoStore) Update(_ context.Context, _ uuid.UUID, _ git.UpdateInput) (git.GitRepository, error) {
	return git.GitRepository{}, errors.New("not implemented")
}

func (s *fakeRepoStore) SoftDelete(_ context.Context, id uuid.UUID) error {
	s.softDeleteCalls++
	if s.softDeleteErr != nil {
		return s.softDeleteErr
	}

	repo, ok := s.repos[id]
	if !ok {
		return git.ErrRepoNotFound
	}
	repo.IsDeleted = true
	s.repos[id] = repo
	return nil
}

func (s *fakeRepoStore) Delete(ctx context.Context, id uuid.UUID) error {
	s.deleteCalls++
	if s.deleteErr != nil {
		return s.deleteErr
	}
	repo, ok := s.repos[id]
	if !ok {
		return git.ErrRepoNotFound
	}
	repo.IsDeleted = true
	s.repos[id] = repo
	return nil
}

func newRepositoryTestMux(repoStore git.Repository, reposDir string, authUserID uuid.UUID) *http.ServeMux {
	nullLogger := logger.NewWithWriter(io.Discard, logger.Config{Level: "error"})
	presenter := NewRepositoryPresenter(repoStore, git.NewService(reposDir), nullLogger)
	mux := http.NewServeMux()
	presenter.RegisterRoutes(mux, Auth(&repoTokenValidator{userID: authUserID}))
	return mux
}

func TestRepositoryPresenter_UnauthorizedRoutes(t *testing.T) {
	store := newFakeRepoStore()
	mux := newRepositoryTestMux(store, t.TempDir(), uuid.New())

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "create repository", method: http.MethodPost, path: "/repos", body: `{"name":"acme"}`},
		{name: "list repositories", method: http.MethodGet, path: "/repos"},
		{name: "delete repository", method: http.MethodDelete, path: "/repos/11111111-1111-1111-1111-111111111111"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
		})
	}
}

func TestRepositoryPresenter_CreateListDeleteFlow(t *testing.T) {
	authUserID := uuid.New()
	store := newFakeRepoStore()
	mux := newRepositoryTestMux(store, t.TempDir(), authUserID)

	createReq := httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(`{"name":"acme-repo","description":"Acme","is_private":true}`))
	createReq.Header.Set("Authorization", "Bearer test-token")
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	mux.ServeHTTP(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}

	var created map[string]any
	if err := json.NewDecoder(createRec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	idRaw, ok := created["id"].(string)
	if !ok || idRaw == "" {
		t.Fatal("created id is empty")
	}

	listReq := httptest.NewRequest(http.MethodGet, "/repos", nil)
	listReq.Header.Set("Authorization", "Bearer test-token")
	listRec := httptest.NewRecorder()
	mux.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}

	var listed []map[string]any
	if err := json.NewDecoder(listRec.Body).Decode(&listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list length = %d, want 1", len(listed))
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/repos/"+idRaw, nil)
	deleteReq.Header.Set("Authorization", "Bearer test-token")
	deleteRec := httptest.NewRecorder()
	mux.ServeHTTP(deleteRec, deleteReq)

	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", deleteRec.Code, http.StatusNoContent)
	}

	listAgainReq := httptest.NewRequest(http.MethodGet, "/repos", nil)
	listAgainReq.Header.Set("Authorization", "Bearer test-token")
	listAgainRec := httptest.NewRecorder()
	mux.ServeHTTP(listAgainRec, listAgainReq)

	if listAgainRec.Code != http.StatusOK {
		t.Fatalf("list-again status = %d, want %d", listAgainRec.Code, http.StatusOK)
	}
	var listedAgain []map[string]any
	if err := json.NewDecoder(listAgainRec.Body).Decode(&listedAgain); err != nil {
		t.Fatalf("decode list-again response: %v", err)
	}
	if len(listedAgain) != 0 {
		t.Fatalf("list-again length = %d, want 0", len(listedAgain))
	}
}

func TestRepositoryPresenter_CreateValidation(t *testing.T) {
	authUserID := uuid.New()
	store := newFakeRepoStore()
	mux := newRepositoryTestMux(store, t.TempDir(), authUserID)

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid json", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "missing name", body: `{"name":""}`, wantStatus: http.StatusBadRequest},
		{name: "invalid name", body: `{"name":"../bad"}`, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestRepositoryPresenter_CreateConflict(t *testing.T) {
	authUserID := uuid.New()
	store := newFakeRepoStore()
	mux := newRepositoryTestMux(store, t.TempDir(), authUserID)

	body := `{"name":"acme-repo"}`
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(body))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if i == 0 && rec.Code != http.StatusCreated {
			t.Fatalf("first create status = %d, want %d", rec.Code, http.StatusCreated)
		}
		if i == 1 && rec.Code != http.StatusConflict {
			t.Fatalf("second create status = %d, want %d", rec.Code, http.StatusConflict)
		}
	}
}

func TestRepositoryPresenter_DeleteValidationAndOwnership(t *testing.T) {
	authUserID := uuid.New()
	otherUserID := uuid.New()
	store := newFakeRepoStore()
	mux := newRepositoryTestMux(store, t.TempDir(), authUserID)

	otherRepo, err := store.Create(context.Background(), git.CreateInput{
		Id:      uuid.New(),
		Name:    "other-repo",
		OwnerID: otherUserID,
	})
	if err != nil {
		t.Fatalf("seed other repo: %v", err)
	}

	t.Run("invalid id", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/repos/not-a-uuid", nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("forbidden for non-owner", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/repos/"+otherRepo.ID.String(), nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
		}
	})

	t.Run("missing repository", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/repos/"+uuid.New().String(), nil)
		req.Header.Set("Authorization", "Bearer test-token")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestRepositoryPresenter_CreateGitFailureRollbackBehavior(t *testing.T) {
	authUserID := uuid.New()

	t.Run("rolls back with soft delete when git create fails", func(t *testing.T) {
		store := newFakeRepoStore()
		brokenRoot := filepath.Join(t.TempDir(), "repos-file")
		if err := os.WriteFile(brokenRoot, []byte("not-a-directory"), 0644); err != nil {
			t.Fatalf("seed broken repo root: %v", err)
		}

		mux := newRepositoryTestMux(store, brokenRoot, authUserID)

		req := httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(`{"name":"rollback-repo"}`))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if store.softDeleteCalls != 1 {
			t.Fatalf("softDeleteCalls = %d, want 1", store.softDeleteCalls)
		}
		if store.deleteCalls != 0 {
			t.Fatalf("deleteCalls = %d, want 0", store.deleteCalls)
		}

		var foundActive bool
		for _, repo := range store.repos {
			if repo.Name == "rollback-repo" && !repo.IsDeleted {
				foundActive = true
			}
		}
		if foundActive {
			t.Fatal("repository remains active after failed git create")
		}
	})

	t.Run("falls back to hard delete when soft delete rollback fails", func(t *testing.T) {
		store := newFakeRepoStore()
		store.softDeleteErr = errors.New("soft delete failed")

		brokenRoot := filepath.Join(t.TempDir(), "repos-file")
		if err := os.WriteFile(brokenRoot, []byte("not-a-directory"), 0644); err != nil {
			t.Fatalf("seed broken repo root: %v", err)
		}

		mux := newRepositoryTestMux(store, brokenRoot, authUserID)

		req := httptest.NewRequest(http.MethodPost, "/repos", bytes.NewBufferString(`{"name":"fallback-repo"}`))
		req.Header.Set("Authorization", "Bearer test-token")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
		if store.softDeleteCalls != 1 {
			t.Fatalf("softDeleteCalls = %d, want 1", store.softDeleteCalls)
		}
		if store.deleteCalls != 1 {
			t.Fatalf("deleteCalls = %d, want 1", store.deleteCalls)
		}
	})
}
