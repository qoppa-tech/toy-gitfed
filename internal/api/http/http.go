// Package http implements a Git Smart HTTP server.
//
// Delegates to the git http-backend CGI process for all pack negotiation.
// This layer is responsible for:
//
//  1. Parsing the request URL and routing to the correct service.
//  2. Setting CGI environment variables expected by git-http-backend(1).
//  3. Forwarding the request body to the backend and the response back.
//
// Supported endpoints:
//
//	GET  /<repo>/info/refs?service=git-upload-pack
//	GET  /<repo>/info/refs?service=git-receive-pack
//	POST /<repo>/git-upload-pack
//	POST /<repo>/git-receive-pack
package http

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	config Config
}

// NewServer creates a new Server with the given configuration.
func NewServer(config Config) *Server {
	return &Server{config: config}
}

// Serve binds to the configured address and blocks, handling connections.
func (s *Server) Serve() error {
	ln, err := net.Listen("tcp", s.config.Address)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	log.Printf("git http-backend listening on %s", s.config.Address)
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

	// Read request body for POST endpoints.
	var body []byte
	if r.Method == "POST" {
		var err error
		body, err = io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	// Invoke git http-backend and relay the CGI response.
	cgiOut, err := s.runGitBackend(parsed, body)
	if err != nil {
		log.Printf("git http-backend error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	s.sendCGIResponse(w, cgiOut, parsed)
}

func (s *Server) runGitBackend(parsed *parsedURL, body []byte) ([]byte, error) {
	env := os.Environ()[:0]

	// Inherit HOME so git can locate its config and SSH keys.
	if home := os.Getenv("HOME"); home != "" {
		env = append(env, "HOME="+home)
	}

	env = append(env,
		"GIT_PROJECT_ROOT="+s.config.ReposDir,
		"GIT_HTTP_EXPORT_ALL=1",
	)

	if len(body) > 0 {
		env = append(env, "REQUEST_METHOD=POST")
	} else {
		env = append(env, "REQUEST_METHOD=GET")
	}

	// PATH_INFO tells git-http-backend which repo and endpoint to serve.
	var suffix string
	switch parsed.Service {
	case infoRefs:
		suffix = "/info/refs"
	case uploadPack:
		suffix = "/git-upload-pack"
	case receivePack:
		suffix = "/git-receive-pack"
	}
	env = append(env, "PATH_INFO=/"+parsed.Repo+suffix)

	if parsed.Query != "" {
		env = append(env, "QUERY_STRING="+parsed.Query)
	}

	if len(body) > 0 {
		env = append(env, "CONTENT_TYPE="+parsed.Service.requestContentType())
		env = append(env, "CONTENT_LENGTH="+strconv.Itoa(len(body)))
	}

	cmd := exec.Command("git", "http-backend")
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(body)
	cmd.Stderr = os.Stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git http-backend: %w", err)
	}
	return out, nil
}

func (s *Server) sendCGIResponse(w http.ResponseWriter, cgiOut []byte, parsed *parsedURL) {
	headersEnd, bodyStart, ok := findHeaderSep(cgiOut)
	if !ok {
		log.Print("git http-backend produced no header/body separator")
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	headersBlock := string(cgiOut[:headersEnd])
	body := cgiOut[bodyStart:]

	status := 200
	contentType := parsed.Service.responseContentType()

	for _, raw := range strings.Split(headersBlock, "\n") {
		line := strings.TrimRight(raw, "\r")
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		value := strings.TrimSpace(line[colon+1:])

		switch strings.ToLower(name) {
		case "status":
			if len(value) >= 3 {
				if code, err := strconv.Atoi(value[:3]); err == nil {
					status = code
				}
			}
		case "content-type":
			contentType = value
		default:
			w.Header().Set(name, value)
		}
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(body)
}

// gitService identifies which Git Smart HTTP endpoint is being requested.
type gitService int

const (
	infoRefs    gitService = iota
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

func (s gitService) requestContentType() string {
	switch s {
	case uploadPack:
		return "application/x-git-upload-pack-request"
	case receivePack:
		return "application/x-git-receive-pack-request"
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

func findHeaderSep(data []byte) (headersEnd, bodyStart int, ok bool) {
	if i := bytes.Index(data, []byte("\r\n\r\n")); i >= 0 {
		return i, i + 4, true
	}
	if i := bytes.Index(data, []byte("\n\n")); i >= 0 {
		return i, i + 2, true
	}
	return 0, 0, false
}
