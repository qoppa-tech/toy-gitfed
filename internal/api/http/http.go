// Package http implements a Git Smart HTTP server.
//
// Delegates to the go-git library for all pack negotiation.
// This layer is responsible for:
//
//  1. Routing requests to the go-git backend handler.
//  2. Validating repository paths before serving.
//  3. Setting correct Content-Type headers per Smart HTTP spec.
//
// Supported endpoints:
//
//	GET  /{repo...}/info/refs?service=git-upload-pack
//	GET  /{repo...}/info/refs?service=git-receive-pack
//	POST /{repo...}/git-upload-pack
//	POST /{repo...}/git-receive-pack
package http

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v6/osfs"
	githttp "github.com/go-git/go-git/v6/backend/http"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/qoppa-tech/gitfed/internal/modules/git"
	"github.com/qoppa-tech/gitfed/internal/modules/organization"
	"github.com/qoppa-tech/gitfed/internal/modules/session"
	"github.com/qoppa-tech/gitfed/internal/modules/sso"
	"github.com/qoppa-tech/gitfed/internal/modules/user"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

// Config holds the server configuration.
type Config struct {
	// Absolute path to directory holding bare git repositories.
	ReposDir string
	// TCP address to listen on (e.g. "0.0.0.0:8080").
	Address string

	// Domain services.
	RepoStore      git.Repository
	GitService     *git.Service
	UserService    *user.Service
	SessionService *session.Service
	SSOService     *sso.Service
	TOTPService    *session.TOTPService
	OrgService     *organization.Service

	// Secure controls whether cookies use the Secure flag.
	Secure bool

	// Rate limiting (nil to disable).
	IPRateLimit   func(http.Handler) http.Handler
	UserRateLimit func(http.Handler) http.Handler

	// Logger (nil to disable).
	Logger logger.Logger
}

// Server is the Git Smart HTTP server.
type Server struct {
	config     Config
	gitHandler *githttp.Backend
	mux        *http.ServeMux
	handler    http.Handler
}

// NewServer creates a new Server with the given configuration.
func NewServer(config Config) *Server {
	fs := osfs.New(config.ReposDir)
	loader := transport.NewFilesystemLoader(fs, false)
	gitHandler := githttp.NewBackend(loader)

	s := &Server{
		config:     config,
		gitHandler: gitHandler,
		mux:        http.NewServeMux(),
	}

	s.registerAuthRoutes()
	s.registerGitRoutes()

	// Build handler chain once: logger → IP rate limit → mux.
	var h http.Handler = s.mux
	if s.config.IPRateLimit != nil {
		h = s.config.IPRateLimit(h)
	}
	if s.config.Logger != nil {
		h = logger.Middleware(s.config.Logger)(h)
	}
	s.handler = h

	return s
}

func (s *Server) registerAuthRoutes() {
	authMw := Auth(s.config.SessionService)

	// Chain: auth -> user rate limit (if configured).
	authChain := func(next http.Handler) http.Handler {
		h := next
		if s.config.UserRateLimit != nil {
			h = s.config.UserRateLimit(h)
		}
		return authMw(h)
	}

	userPresenter := NewUserPresenter(s.config.UserService)
	sessionPresenter := NewSessionPresenter(s.config.SessionService, s.config.UserService)
	sessionPresenter.SetSecure(s.config.Secure)
	ssoPresenter := NewSSOPresenter(s.config.SSOService, s.config.UserService, s.config.SessionService)
	ssoPresenter.SetSecure(s.config.Secure)
	totpPresenter := NewTOTPPresenter(s.config.TOTPService)
	organizationPresenter := NewOrganizationPresenter(s.config.OrgService)
	repositoryPresenter := NewRepositoryPresenter(s.config.RepoStore, s.config.GitService, s.config.Logger)

	userPresenter.RegisterRoutes(s.mux)
	sessionPresenter.RegisterRoutes(s.mux)
	ssoPresenter.RegisterRoutes(s.mux)
	totpPresenter.RegisterRoutes(s.mux, authChain)
	organizationPresenter.RegisterRoutes(s.mux, authMw)
	repositoryPresenter.RegisterRoutes(s.mux, authMw)
}

func (s *Server) registerGitRoutes() {
	// Method-specific catch-alls. More specific auth routes take precedence.
	s.mux.HandleFunc("GET /{path...}", s.handleGitGet)
	s.mux.HandleFunc("POST /{path...}", s.handleGitPost)
}

func (s *Server) handleGitGet(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	repo, ok := strings.CutSuffix(path, "/info/refs")
	if !ok || repo == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(r.URL.Query().Get("service"), "git-") {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	s.serveGit(w, r, repo)
}

func (s *Server) handleGitPost(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")

	var repo string
	switch {
	case strings.HasSuffix(path, "/git-upload-pack"):
		repo, _ = strings.CutSuffix(path, "/git-upload-pack")
	case strings.HasSuffix(path, "/git-receive-pack"):
		repo, _ = strings.CutSuffix(path, "/git-receive-pack")
	}
	if repo == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}
	s.serveGit(w, r, repo)
}

func (s *Server) serveGit(w http.ResponseWriter, r *http.Request, repo string) {
	repoPath, ok := secureRepoPath(s.config.ReposDir, repo)
	if !ok {
		http.Error(w, "Repository Not Found", http.StatusNotFound)
		return
	}

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		http.Error(w, "Repository Not Found", http.StatusNotFound)
		return
	}
	s.gitHandler.ServeHTTP(w, r)
}

func secureRepoPath(reposDir, repo string) (string, bool) {
	if repo == "" || filepath.IsAbs(repo) {
		return "", false
	}

	cleanRepo := filepath.Clean(repo)
	if cleanRepo == "." || cleanRepo == ".." || strings.HasPrefix(cleanRepo, ".."+string(filepath.Separator)) {
		return "", false
	}

	baseAbs, err := filepath.Abs(reposDir)
	if err != nil {
		return "", false
	}
	baseReal, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", false
	}

	repoAbs, err := filepath.Abs(filepath.Join(baseAbs, cleanRepo))
	if err != nil {
		return "", false
	}

	if repoAbs != baseAbs && !strings.HasPrefix(repoAbs, baseAbs+string(filepath.Separator)) {
		return "", false
	}

	repoReal, err := filepath.EvalSymlinks(repoAbs)
	if err == nil {
		if repoReal != baseReal && !strings.HasPrefix(repoReal, baseReal+string(filepath.Separator)) {
			return "", false
		}
	} else if !os.IsNotExist(err) {
		return "", false
	}

	return repoAbs, true
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Serve binds to the configured address and blocks, handling connections.
func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	if s.config.Logger != nil {
		s.config.Logger.Info("server listening", "address", s.config.Address)
	}
	return http.Serve(ln, s)
}
