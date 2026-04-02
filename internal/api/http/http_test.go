package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		path    string
		query   string
		wantNil bool
		want    *parsedURL
	}{
		{
			name:  "InfoRefs upload-pack",
			path:  "/myrepo.git/info/refs",
			query: "service=git-upload-pack",
			want: &parsedURL{
				Repo:    "myrepo.git",
				Service: infoRefs,
				Query:   "service=git-upload-pack",
			},
		},
		{
			name:  "InfoRefs receive-pack",
			path:  "/myrepo.git/info/refs",
			query: "service=git-receive-pack",
			want: &parsedURL{
				Repo:    "myrepo.git",
				Service: infoRefs,
				Query:   "service=git-receive-pack",
			},
		},
		{
			name:  "UploadPack POST",
			path:  "/myrepo.git/git-upload-pack",
			query: "",
			want: &parsedURL{
				Repo:    "myrepo.git",
				Service: uploadPack,
			},
		},
		{
			name:  "ReceivePack POST",
			path:  "/myrepo.git/git-receive-pack",
			query: "",
			want: &parsedURL{
				Repo:    "myrepo.git",
				Service: receivePack,
			},
		},
		{
			name:  "Nested repo path",
			path:  "/org/team/repo.git/git-upload-pack",
			query: "",
			want: &parsedURL{
				Repo:    "org/team/repo.git",
				Service: uploadPack,
			},
		},
		{
			name:    "Root path",
			path:    "/",
			query:   "",
			wantNil: true,
		},
		{
			name:    "InfoRefs without service query",
			path:    "/repo/info/refs",
			query:   "",
			wantNil: true,
		},
		{
			name:    "Unknown path",
			path:    "/repo/objects/info/packs",
			query:   "",
			wantNil: true,
		},
		{
			name:    "Empty repo in InfoRefs",
			path:    "/info/refs",
			query:   "service=git-upload-pack",
			wantNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			p := parseGitURL(tc.path, tc.query)
			if tc.wantNil {
				if p != nil {
					t.Errorf("parseGitURL(%q, %q) = %+v, want nil", tc.path, tc.query, p)
				}
				return
			}
			if p == nil {
				t.Fatalf("parseGitURL(%q, %q) = nil, want %+v", tc.path, tc.query, tc.want)
			}
			if p.Repo != tc.want.Repo {
				t.Errorf("repo = %q, want %q", p.Repo, tc.want.Repo)
			}
			if p.Service != tc.want.Service {
				t.Errorf("service = %v, want %v", p.Service, tc.want.Service)
			}
			if p.Query != tc.want.Query {
				t.Errorf("query = %q, want %q", p.Query, tc.want.Query)
			}
		})
	}
}

func TestServeHTTP_InfoRefs(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "test-repo.git")
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatal(err)
	}
	// Initialize a bare git repository
	if err := runGit(repoPath, "init", "--bare"); err != nil {
		t.Skipf("git not available: %v", err)
	}

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0"})

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
			name:       "POST to info/refs not allowed",
			url:        "/test-repo.git/info/refs?service=git-upload-pack",
			method:     http.MethodPost,
			wantStatus: http.StatusMethodNotAllowed,
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

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0"})

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
			body:        "0000", // flush packet
			contentType: "application/x-git-upload-pack-request",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "GET to git-upload-pack not allowed",
			url:        "/test-repo.git/git-upload-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
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

	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0"})

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
			body:        "0000", // flush packet
			contentType: "application/x-git-receive-pack-request",
			wantStatus:  http.StatusOK,
		},
		{
			name:       "GET to git-receive-pack not allowed",
			url:        "/test-repo.git/git-receive-pack",
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
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
	srv := NewServer(Config{ReposDir: tmpDir, Address: "127.0.0.1:0"})

	tests := []struct {
		name       string
		url        string
		method     string
		body       string
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
			name:       "Empty path",
			url:        "/",
			method:     http.MethodGet,
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(tc.method, tc.url, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			srv.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

// runGit is a helper to run git commands for test setup.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
