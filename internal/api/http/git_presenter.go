package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/qoppa-tech/gitfed/internal/modules/git"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type GitPresenter struct {
	svc *git.Service
}

func NewGitPresenter(svc *git.Service) *GitPresenter {
	return &GitPresenter{svc: svc}
}

func (p *GitPresenter) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{path...}", p.handleGitGet)
	mux.HandleFunc("POST /{path...}", p.handleGitPost)
}

func (p *GitPresenter) handleGitGet(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	repo, ok := strings.CutSuffix(path, "/info/refs")
	if !ok || repo == "" {
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	svc := r.URL.Query().Get("service")
	if svc != "git-upload-pack" && svc != "git-receive-pack" {
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	repository := git.GitRepository{Name: repoNameFromPath(repo)}
	if !p.svc.Exists(repository) {
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", svc))
	w.Header().Set("Cache-Control", "no-cache")

	ctx := r.Context()
	req := git.UploadPackRequest{
		RepoPath:     repo,
		Adverts:      true,
		StatelessRPC: true,
	}

	if err := p.svc.UploadPack(ctx, req, w, http.NoBody); err != nil {
		logger.FromContext(ctx).Error("git upload-pack failed", "step", "upload_pack_advert", "repo", repo, "error", err)
		writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "upload-pack failed"})
		return
	}
}

func (p *GitPresenter) handleGitPost(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	var repo string
	switch {
	case strings.HasSuffix(path, "/git-upload-pack"):
		repo, _ = strings.CutSuffix(path, "/git-upload-pack")
	case strings.HasSuffix(path, "/git-receive-pack"):
		repo, _ = strings.CutSuffix(path, "/git-receive-pack")
	default:
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if repo == "" {
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	repository := git.GitRepository{Name: repoNameFromPath(repo)}
	if !p.svc.Exists(repository) {
		writeJSON(r.Context(), w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}

	ctx := r.Context()

	if strings.HasSuffix(path, "/git-upload-pack") {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-git-upload-pack-request" {
			writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid content-type"})
			return
		}

		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")

		req := git.UploadPackRequest{
			RepoPath:     repo,
			StatelessRPC: true,
		}

		if err := p.svc.UploadPack(ctx, req, w, r.Body); err != nil {
			logger.FromContext(ctx).Error("git upload-pack failed", "step", "upload_pack", "repo", repo, "error", err)
			writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "upload-pack failed"})
			return
		}
	} else {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-git-receive-pack-request" {
			writeJSON(r.Context(), w, http.StatusBadRequest, map[string]string{"error": "invalid content-type"})
			return
		}

		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

		req := git.ReceivePackRequest{
			RepoPath:     repo,
			StatelessRPC: true,
		}

		if err := p.svc.ReceivePack(ctx, req, w, r.Body); err != nil {
			logger.FromContext(ctx).Error("git receive-pack failed", "step", "receive_pack", "repo", repo, "error", err)
			writeJSON(r.Context(), w, http.StatusInternalServerError, map[string]string{"error": "receive-pack failed"})
			return
		}
	}
}

func repoNameFromPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return path
}

type errorResponse struct {
	Error string `json:"error"`
}
