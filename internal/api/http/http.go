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
	"log"
	"net"
	"net/http"

	"github.com/qoppa-tech/toy-gitfed/internal/modules/git"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/organization"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/session"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/sso"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/user"
)

// Config holds the server configuration.
type Config struct {
	// Absolute path to directory holding bare git repositories.
	ReposDir string
	// TCP address to listen on (e.g. "0.0.0.0:8080").
	Address string

	// Domain services.
	GitService     *git.Service
	UserService    *user.Service
	SessionService *session.Service
	SSOService     *sso.Service
	TOTPService    *session.TOTPService
	OrgService     *organization.Service

	// Secure controls whether cookies use the Secure flag.
	Secure bool
}

// Server is the Git Smart HTTP server.
type Server struct {
	config Config
	mux    *http.ServeMux
}

// NewServer creates a new Server with the given configuration.
func NewServer(config Config) *Server {
	s := &Server{
		config: config,
		mux:    http.NewServeMux(),
	}

	s.registerAuthRoutes()
	s.registerGitRoutes()
	return s
}

func (s *Server) registerAuthRoutes() {
	authMw := Auth(s.config.SessionService)

	userPresenter := NewUserPresenter(s.config.UserService)
	sessionPresenter := NewSessionPresenter(s.config.SessionService, s.config.UserService)
	sessionPresenter.SetSecure(s.config.Secure)
	ssoPresenter := NewSSOPresenter(s.config.SSOService, s.config.UserService, s.config.SessionService)
	ssoPresenter.SetSecure(s.config.Secure)
	totpPresenter := NewTOTPPresenter(s.config.TOTPService)

	userPresenter.RegisterRoutes(s.mux)
	sessionPresenter.RegisterRoutes(s.mux)
	ssoPresenter.RegisterRoutes(s.mux)
	totpPresenter.RegisterRoutes(s.mux, authMw)
}

func (s *Server) registerGitRoutes() {
	gitPresenter := NewGitPresenter(s.config.GitService)
	gitPresenter.RegisterRoutes(s.mux)
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
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
