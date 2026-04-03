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
//	GET  /<repo>/info/refs?service=git-upload-pack
//	GET  /<repo>/info/refs?service=git-receive-pack
//	POST /<repo>/git-upload-pack
//	POST /<repo>/git-receive-pack
package http

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v6/osfs"
	githttp "github.com/go-git/go-git/v6/backend/http"
	"github.com/go-git/go-git/v6/plumbing/transport"

	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
	"github.com/qoppa-tech/toy-gitfed/internal/server/middleware"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/organization"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/session"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/sso"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/user"
	"github.com/qoppa-tech/toy-gitfed/internal/store"
)

// Config holds the server configuration.
type Config struct {
	// Absolute path to directory holding bare git repositories.
	ReposDir string
	// TCP address to listen on (e.g. "0.0.0.0:8080").
	Address string
	// Queries is the sqlc Queries instance for database access.
	Queries *sqlc.Queries
	// Redis is the Redis store for session/token management.
	Redis *store.RedisStore
	// Google OAuth configuration.
	GoogleOAuth sso.GoogleConfig
	// TOTPIssuer is the issuer name shown in authenticator apps.
	TOTPIssuer string
}

// Server is the Git Smart HTTP server.
type Server struct {
	config     Config
	gitHandler *githttp.Backend
	mux        *http.ServeMux
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
	return s
}

func (s *Server) registerAuthRoutes() {
	q := s.config.Queries
	redis := s.config.Redis

	// Services.
	userSvc := user.NewService(q)
	sessionSvc := session.NewService(q, redis)
	ssoSvc := sso.NewService(q, redis, s.config.GoogleOAuth)
	totpSvc := session.NewTOTPService(redis, s.config.TOTPIssuer)
	_ = organization.NewService(q) // scaffold — routes can be added later

	// Auth middleware.
	authMw := middleware.Auth(sessionSvc)

	// Handlers.
	userHandler := user.NewHandler(userSvc)
	sessionHandler := session.NewHandler(sessionSvc, userSvc)
	ssoHandler := sso.NewHandler(ssoSvc, userSvc, sessionSvc)
	totpHandler := session.NewTOTPHandler(totpSvc)

	// Register routes.
	userHandler.RegisterRoutes(s.mux)
	sessionHandler.RegisterRoutes(s.mux)
	ssoHandler.RegisterRoutes(s.mux)
	totpHandler.RegisterRoutes(s.mux, authMw)
}

// Serve binds to the configured address and blocks, handling connections.
func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	log.Printf("git http server listening on %s", s.config.Address)
	return http.Serve(ln, s)
}

// ServeHTTP implements http.Handler.
// Auth routes are handled by the mux; git routes by the go-git backend.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Route /auth/* requests to the auth mux.
	if strings.HasPrefix(r.URL.Path, "/auth/") {
		s.mux.ServeHTTP(w, r)
		return
	}

	// Git Smart HTTP handling.
	parsed := parseGitURL(r.URL.Path, r.URL.RawQuery)
	if parsed == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	wantMethod := "GET"
	if parsed.Service == uploadPack || parsed.Service == receivePack {
		wantMethod = "POST"
	}
	if r.Method != wantMethod {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify the repository exists on disk.
	repoPath := filepath.Join(s.config.ReposDir, parsed.Repo)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		http.Error(w, "Repository Not Found", http.StatusNotFound)
		return
	}

	// Construct the URL path expected by the go-git handler.
	var suffix string
	switch parsed.Service {
	case infoRefs:
		suffix = "/info/refs"
	case uploadPack:
		suffix = "/git-upload-pack"
	case receivePack:
		suffix = "/git-receive-pack"
	}
	r.URL.Path = "/" + parsed.Repo + suffix

	// Delegate to the go-git backend handler.
	s.gitHandler.ServeHTTP(w, r)
}

// gitService identifies which Git Smart HTTP endpoint is being requested.
// WARN: LEGACY
type gitService int

const (
	infoRefs gitService = iota
	uploadPack
	receivePack
)

func (s gitService) responseContentType() string {
	switch s {
	case infoRefs:
		return "application/x-git-upload-pack-advertisement"
	case uploadPack:
		return "application/x-git-upload-pack-result"
	case receivePack:
		return "application/x-git-receive-pack-result"
	default:
		return "application/octet-stream"
	}
}

type parsedURL struct {
	Repo    string
	Service gitService
	Query   string
}

// parseGitURL parses a Git Smart HTTP URL target into its components.
func parseGitURL(path, query string) *parsedURL {
	if len(path) < 2 || path[0] != '/' {
		return nil
	}
	p := path[1:] // strip leading '/'

	if strings.HasSuffix(p, "/info/refs") {
		repo := p[:len(p)-len("/info/refs")]
		if repo == "" {
			return nil
		}
		if !strings.HasPrefix(query, "service=git-") {
			return nil
		}
		return &parsedURL{Repo: repo, Service: infoRefs, Query: query}
	}

	if strings.HasSuffix(p, "/git-upload-pack") {
		repo := p[:len(p)-len("/git-upload-pack")]
		if repo == "" {
			return nil
		}
		return &parsedURL{Repo: repo, Service: uploadPack}
	}

	if strings.HasSuffix(p, "/git-receive-pack") {
		repo := p[:len(p)-len("/git-receive-pack")]
		if repo == "" {
			return nil
		}
		return &parsedURL{Repo: repo, Service: receivePack}
	}

	return nil
}
