// Package http implements a Git Smart HTTP server.
//
// Delegates to the go-git-http library for all pack negotiation.
// This layer is responsible for:
//
//  1. Routing requests to the go-git-http handler.
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

	githttp "github.com/AaronO/go-git-http"
)

// Config holds the server configuration.
type Config struct {
	// Absolute path to directory holding bare git repositories.
	ReposDir string
	// TCP address to listen on (e.g. "0.0.0.0:8080").
	Address string
}

// Server is the Git Smart HTTP server.
type Server struct {
	config  Config
	handler *githttp.GitHttp
}

// NewServer creates a new Server with the given configuration.
func NewServer(config Config) *Server {
	gitHandler := githttp.New(config.ReposDir)
	return &Server{
		config:  config,
		handler: gitHandler,
	}
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
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Construct the URL path expected by the go-git-http handler.
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

	// Delegate to the go-git-http handler.
	s.handler.ServeHTTP(w, r)
}

// gitService identifies which Git Smart HTTP endpoint is being requested.
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
