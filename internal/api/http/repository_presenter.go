package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/qoppa-tech/gitfed/internal/modules/git"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type RepositoryPresenter struct {
	store      git.Repository
	gitService *git.Service
	logger     logger.Logger
}

func NewRepositoryPresenter(store git.Repository, gitService *git.Service, logger logger.Logger) *RepositoryPresenter {
	return &RepositoryPresenter{store: store, gitService: gitService, logger: logger}
}

func (p *RepositoryPresenter) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /repos", authMw(http.HandlerFunc(p.Create)))
	mux.Handle("GET /repos", authMw(http.HandlerFunc(p.List)))
	mux.Handle("DELETE /repos/{id}", authMw(http.HandlerFunc(p.Delete)))
}

type createRepositoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
	DefaultRef  string `json:"default_ref"`
}

type repositoryResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	IsPrivate   bool   `json:"is_private"`
	OwnerID     string `json:"owner_id"`
	DefaultRef  string `json:"default_ref,omitempty"`
}

func (p *RepositoryPresenter) Create(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createRepositoryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.DefaultRef = strings.TrimSpace(req.DefaultRef)

	if req.Name == "" {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := git.ValidateRepoName(req.Name); err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid repository name"})
		return
	}

	repoID := uuid.New()
	repo, err := p.store.Create(r.Context(), git.CreateInput{
		Id:          repoID,
		Name:        req.Name,
		Description: req.Description,
		IsPrivate:   req.IsPrivate,
		OwnerID:     authUserID,
		DefaultRef:  req.DefaultRef,
	})
	if err != nil {
		if errors.Is(err, git.ErrRepoAlreadyExists) {
			writeJSON(r.Context(), w, http.StatusConflict, map[string]string{"error": "repository already exists"})
			return
		}
		p.logger.Error("repository store create failed", "step", "repo_store_create", "repo_name", req.Name, "owner_id", authUserID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if _, err := p.gitService.Create(r.Context(), git.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		IsPrivate:   req.IsPrivate,
		OwnerID:     authUserID,
		DefaultRef:  req.DefaultRef,
	}); err != nil {
		p.logger.Error("git service create failed", "step", "git_create", "repo_id", repo.ID.String(), "repo_name", req.Name, "error", err)
		if rollbackErr := p.store.SoftDelete(r.Context(), repo.ID); rollbackErr != nil {
			if hardDeleteErr := p.store.Delete(r.Context(), repo.ID); hardDeleteErr != nil {
				p.logger.Error("repository rollback failed after git create error", "step", "repo_rollback", "repo_id", repo.ID.String(), "soft_delete_error", rollbackErr, "hard_delete_error", hardDeleteErr)
			}
		}
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(r.Context(), w, http.StatusCreated, toRepositoryResponse(repo))
}

func (p *RepositoryPresenter) List(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	repos, err := p.store.ListByOwner(r.Context(), authUserID)
	if err != nil {
		p.logger.Error("repository list failed", "step", "repo_list", "owner_id", authUserID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]repositoryResponse, len(repos))
	for i := range repos {
		resp[i] = toRepositoryResponse(repos[i])
	}
	writeJSON(r.Context(), w, http.StatusOK, resp)
}

func (p *RepositoryPresenter) Delete(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(r.Context(), w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	repoID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}

	repo, err := p.store.GetByID(r.Context(), repoID)
	if err != nil {
		if errors.Is(err, git.ErrRepoNotFound) {
			writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "repository not found"})
			return
		}
		p.logger.Error("repository get failed", "step", "repo_get", "repo_id", repoID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if repo.OwnerID != authUserID {
		writeJSON(r.Context(), w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := p.store.SoftDelete(r.Context(), repoID); err != nil {
		if errors.Is(err, git.ErrRepoNotFound) {
			writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "repository not found"})
			return
		}
		p.logger.Error("repository soft delete failed", "step", "repo_soft_delete", "repo_id", repoID.String(), "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func toRepositoryResponse(repo git.GitRepository) repositoryResponse {
	return repositoryResponse{
		ID:          repo.ID.String(),
		Name:        repo.Name,
		Description: repo.Description,
		IsPrivate:   repo.IsPrivate,
		OwnerID:     repo.OwnerID.String(),
		DefaultRef:  repo.DefaultRef,
	}
}
