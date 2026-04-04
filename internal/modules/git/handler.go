package git

import (
	"fmt"
	"net/http"
	"strings"
)

// SmartHTTPHandler provides http.Handler implementations for Git Smart HTTP endpoints.
type SmartHTTPHandler struct {
	svc *Service
	mux *http.ServeMux
}

// NewSmartHTTPHandler creates a new handler wired to the given service.
func NewSmartHTTPHandler(svc *Service) *SmartHTTPHandler {
	h := &SmartHTTPHandler{
		svc: svc,
		mux: http.NewServeMux(),
	}
	h.registerRoutes()
	return h
}

func (h *SmartHTTPHandler) registerRoutes() {
	h.mux.HandleFunc("GET /{path...}", h.handleGitGet)
	h.mux.HandleFunc("POST /{path...}", h.handleGitPost)
}

func (h *SmartHTTPHandler) handleGitGet(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	repo, ok := strings.CutSuffix(path, "/info/refs")
	if !ok || repo == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	svc := r.URL.Query().Get("service")
	if svc != "git-upload-pack" && svc != "git-receive-pack" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	repository := Repository{Name: repoNameFromPath(repo)}
	if !h.svc.Exists(repository) {
		http.Error(w, "Repository Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", svc))
	w.Header().Set("Cache-Control", "no-cache")

	ctx := r.Context()
	req := UploadPackRequest{
		RepoPath:     repo,
		Adverts:      true,
		StatelessRPC: true,
	}

	if err := h.svc.UploadPack(ctx, req, w, http.NoBody); err != nil {
		http.Error(w, fmt.Sprintf("upload-pack: %v", err), http.StatusInternalServerError)
		return
	}
}

func (h *SmartHTTPHandler) handleGitPost(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	var repo string
	switch {
	case strings.HasSuffix(path, "/git-upload-pack"):
		repo, _ = strings.CutSuffix(path, "/git-upload-pack")
	case strings.HasSuffix(path, "/git-receive-pack"):
		repo, _ = strings.CutSuffix(path, "/git-receive-pack")
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if repo == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	repository := Repository{Name: repoNameFromPath(repo)}
	if !h.svc.Exists(repository) {
		http.Error(w, "Repository Not Found", http.StatusNotFound)
		return
	}

	ctx := r.Context()

	if strings.HasSuffix(path, "/git-upload-pack") {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-git-upload-pack-request" {
			http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/x-git-upload-pack-result")

		req := UploadPackRequest{
			RepoPath:     repo,
			StatelessRPC: true,
		}

		if err := h.svc.UploadPack(ctx, req, w, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("upload-pack: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-git-receive-pack-request" {
			http.Error(w, "Invalid Content-Type", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/x-git-receive-pack-result")

		req := ReceivePackRequest{
			RepoPath:     repo,
			StatelessRPC: true,
		}

		if err := h.svc.ReceivePack(ctx, req, w, r.Body); err != nil {
			http.Error(w, fmt.Sprintf("receive-pack: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

// ServeHTTP implements http.Handler.
func (h *SmartHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

// ServeMux returns the underlying ServeMux for integration with larger routers.
func (h *SmartHTTPHandler) ServeMux() *http.ServeMux {
	return h.mux
}

// Mount attaches the Smart HTTP handlers to an existing ServeMux.
func (h *SmartHTTPHandler) Mount(mux *http.ServeMux) {
	mux.HandleFunc("GET /{path...}", h.handleGitGet)
	mux.HandleFunc("POST /{path...}", h.handleGitPost)
}

// ValidateRepoName checks if a repository name is valid.
func ValidateRepoName(name string) error {
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("%w: %q", ErrInvalidRepoName, name)
	}
	return nil
}

// SanitizeRepoPath removes path traversal components from a repo path.
func SanitizeRepoPath(path string) string {
	path = strings.TrimPrefix(path, "/")
	path = strings.ReplaceAll(path, "..", "")
	path = strings.Trim(path, "/")
	return path
}

// BuildRepoPath constructs a safe repository path from components.
func BuildRepoPath(base string, components ...string) string {
	parts := []string{base}
	for _, c := range components {
		parts = append(parts, SanitizeRepoPath(c))
	}
	return strings.Join(parts, "/")
}
