package http

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/git"
)

type RepositoryPresenter struct {
	store      git.Repository
	gitService *git.Service
}

func NewRepositoryPresenter(store git.Repository, gitService *git.Service) *RepositoryPresenter {
	return &RepositoryPresenter{store: store, gitService: gitService}
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
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createRepositoryRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.DefaultRef = strings.TrimSpace(req.DefaultRef)

	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := git.ValidateRepoName(req.Name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository name"})
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
			writeJSON(w, http.StatusConflict, map[string]string{"error": "repository already exists"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	if _, err := p.gitService.Create(r.Context(), git.CreateInput{
		Name:        req.Name,
		Description: req.Description,
		IsPrivate:   req.IsPrivate,
		OwnerID:     authUserID,
		DefaultRef:  req.DefaultRef,
	}); err != nil {
		if rollbackErr := p.store.SoftDelete(r.Context(), repo.ID); rollbackErr != nil {
			if hardDeleteErr := p.store.Delete(r.Context(), repo.ID); hardDeleteErr != nil {
				log.Printf("repository rollback failed after git create error: soft-delete=%v hard-delete=%v", rollbackErr, hardDeleteErr)
			}
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusCreated, toRepositoryResponse(repo))
}

func (p *RepositoryPresenter) List(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	repos, err := p.store.ListByOwner(r.Context(), authUserID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	resp := make([]repositoryResponse, len(repos))
	for i := range repos {
		resp[i] = toRepositoryResponse(repos[i])
	}
	writeJSON(w, http.StatusOK, resp)
}

func (p *RepositoryPresenter) Delete(w http.ResponseWriter, r *http.Request) {
	authUserID, ok := UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	repoID, err := uuid.Parse(strings.TrimSpace(r.PathValue("id")))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid repository id"})
		return
	}

	repo, err := p.store.GetByID(r.Context(), repoID)
	if err != nil {
		if errors.Is(err, git.ErrRepoNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}
	if repo.OwnerID != authUserID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if err := p.store.SoftDelete(r.Context(), repoID); err != nil {
		if errors.Is(err, git.ErrRepoNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
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
