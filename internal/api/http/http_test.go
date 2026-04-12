package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/qoppa-tech/toy-gitfed/internal/modules/git"
)

func TestServeHTTP_InfoRefs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo.git")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repoPath, "init", "--bare"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0", GitService: git.NewService(tmpDir)})

	tests := []struct {
		name            string
		url             string
		method          string
		wantStatus      int
		wantContentType string
	}{
		{
			name:            "GET info/refs upload-pack",
			url:             "/test-repo.git/info/refs?service=git-upload-pack",
			method:          http.MethodGet,
			wantStatus:      http.StatusOK,
			wantContentType: "application/x-git-upload-pack-advertisement",
		},
		{
			name:            "GET info/refs receive-pack",
			url:             "/test-repo.git/info/refs?service=git-receive-pack",
			method:          http.MethodGet,
			wantStatus:      http.StatusOK,
			wantContentType: "application/x-git-receive-pack-advertisement",
		},
		{
			name:       "POST to info/refs not found",
			url:        "/test-repo.git/info/refs?service=git-upload-pack",
			method:     http.MethodPost,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "GET info/refs without service query",
			url:        "/test-repo.git/info/refs",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.url, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantContentType != "" {
				ct := rec.Header().Get("Content-Type")
				if ct != tc.wantContentType {
					t.Errorf("content-type = %q, want %q", ct, tc.wantContentType)
				}
			}
		})
	}
}

func TestServeHTTP_UploadPack(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo.git")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repoPath, "init", "--bare"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0", GitService: git.NewService(tmpDir)})

	tests := []struct {
		name        string
		url         string
		method      string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "POST git-upload-pack with valid request",
			url:         "/test-repo.git/git-upload-pack",
			method:      http.MethodPost,
			body:        "0000",
			contentType: "application/x-git-upload-pack-request",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "GET to git-upload-pack not found",
			url:        "/test-repo.git/git-upload-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestServeHTTP_ReceivePack(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo.git")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(repoPath, "init", "--bare"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0", GitService: git.NewService(tmpDir)})

	tests := []struct {
		name        string
		url         string
		method      string
		body        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "POST git-receive-pack with valid request",
			url:         "/test-repo.git/git-receive-pack",
			method:      http.MethodPost,
			body:        "0000",
			contentType: "application/x-git-receive-pack-request",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "GET to git-receive-pack not found",
			url:        "/test-repo.git/git-receive-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestServeHTTP_ErrorCases(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	escapeRoot := t.TempDir()
	escapeRepoPath := filepath.Join(escapeRoot, "escape.git")
	if err := os.MkdirAll(escapeRepoPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := runGit(escapeRepoPath, "init", "--bare"); err != nil {
		t.Skipf("git not available: %v", err)
	}
	if err := os.Symlink(escapeRepoPath, filepath.Join(tmpDir, "escape.git")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0", GitService: git.NewService(tmpDir)})

	tests := []struct {
		name       string
		url        string
		method     string
		wantStatus int
	}{
		{
			name:       "Non-existent repository",
			url:        "/nonexistent.git/info/refs?service=git-upload-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Unknown path",
			url:        "/random/path",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Path traversal repository blocked",
			url:        "/%2e%2e/etc/info/refs?service=git-upload-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Symlink escape repository blocked",
			url:        "/escape.git/info/refs?service=git-upload-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "Empty path",
			url:        "/",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.url, nil)
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
