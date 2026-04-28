package http

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/qoppa-tech/gitfed/internal/modules/git"
)

type mockGitService struct {
	exists bool
	err    error
}

func (m *mockGitService) Exists(repo git.GitRepository) bool {
	return m.exists
}

func (m *mockGitService) UploadPack(ctx interface{}, req interface{}, w interface{}, body io.Reader) error {
	return m.err
}

func (m *mockGitService) ReceivePack(ctx interface{}, req interface{}, w interface{}, body io.Reader) error {
	return m.err
}

func TestRepoNameFromPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantName string
	}{
		{
			name:     "simple repo name",
			path:     "myrepo",
			wantName: "myrepo",
		},
		{
			name:     "nested path",
			path:     "owner/repo",
			wantName: "repo",
		},
		{
			name:     "deeply nested path",
			path:     "a/b/c/d",
			wantName: "d",
		},
		{
			name:     "single character",
			path:     "x",
			wantName: "x",
		},
		{
			name:     "path with trailing slash",
			path:     "repo/",
			wantName: "",
		},
		{
			name:     "empty path",
			path:     "",
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := repoNameFromPath(tt.path)
			if got != tt.wantName {
				t.Errorf("repoNameFromPath(%q) = %q, want %q", tt.path, got, tt.wantName)
			}
		})
	}
}

func TestHandleGitGet(t *testing.T) {
	tests := []struct {
		name            string
		path            string
		query           string
		mockExists      bool
		wantStatus      int
		wantContentType string
	}{
		{
			name:            "missing /info/refs suffix",
			path:            "myrepo",
			query:           "service=git-upload-pack",
			mockExists:      true,
			wantStatus:      http.StatusNotFound,
			wantContentType: "",
		},
		{
			name:            "empty repo after suffix cut",
			path:            "/info/refs",
			query:           "service=git-upload-pack",
			mockExists:      true,
			wantStatus:      http.StatusNotFound,
			wantContentType: "",
		},
		{
			name:            "missing service query param",
			path:            "myrepo/info/refs",
			query:           "",
			mockExists:      true,
			wantStatus:      http.StatusNotFound,
			wantContentType: "",
		},
		{
			name:            "invalid service",
			path:            "myrepo/info/refs",
			query:           "service=git-foo",
			mockExists:      true,
			wantStatus:      http.StatusNotFound,
			wantContentType: "",
		},
		{
			name:            "repository does not exist",
			path:            "myrepo/info/refs",
			query:           "service=git-upload-pack",
			mockExists:      false,
			wantStatus:      http.StatusNotFound,
			wantContentType: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockGitService{exists: tt.mockExists}
			_ = svc // suppress unused warning
			presenter := &GitPresenter{svc: (*git.Service)(nil)}

			url := "/" + tt.path
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest("GET", url, nil)
			w := httptest.NewRecorder()

			presenter.handleGitGet(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleGitPost(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		contentType string
		mockExists  bool
		wantStatus  int
	}{
		{
			name:        "missing upload-pack or receive-pack suffix",
			path:        "myrepo",
			contentType: "application/x-git-upload-pack-request",
			mockExists:  true,
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "empty repo after upload-pack suffix",
			path:        "/git-upload-pack",
			contentType: "application/x-git-upload-pack-request",
			mockExists:  true,
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "empty repo after receive-pack suffix",
			path:        "/git-receive-pack",
			contentType: "application/x-git-receive-pack-request",
			mockExists:  true,
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "repository does not exist for upload-pack",
			path:        "myrepo/git-upload-pack",
			contentType: "application/x-git-upload-pack-request",
			mockExists:  false,
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "repository does not exist for receive-pack",
			path:        "myrepo/git-receive-pack",
			contentType: "application/x-git-receive-pack-request",
			mockExists:  false,
			wantStatus:  http.StatusNotFound,
		},
		{
			name:        "invalid content type for upload-pack",
			path:        "myrepo/git-upload-pack",
			contentType: "application/json",
			mockExists:  true,
			wantStatus:  http.StatusNotFound, // svc is nil so Exists check fails
		},
		{
			name:        "invalid content type for receive-pack",
			path:        "myrepo/git-receive-pack",
			contentType: "application/json",
			mockExists:  true,
			wantStatus:  http.StatusNotFound, // svc is nil so Exists check fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &mockGitService{exists: tt.mockExists}
			_ = svc // suppress unused warning
			presenter := &GitPresenter{svc: (*git.Service)(nil)}

			req := httptest.NewRequest("POST", "/"+tt.path, nil)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}
			w := httptest.NewRecorder()

			presenter.handleGitPost(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}
