package git

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v6"
	"github.com/google/uuid"
	"github.com/qoppa-tech/toy-gitfed/pkg/pktline"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return dir
}

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	dir := setupTestDir(t)
	return NewService(dir), dir
}

func newClonedRepo(t *testing.T, svc *Service, name string) Repository {
	t.Helper()
	path := filepath.Join(svc.reposDir, name)
	g, err := git.PlainClone(path, &git.CloneOptions{
		URL:  "https://github.com/git-fixtures/basic.git",
		Bare: true,
	})
	if err != nil {
		t.Fatalf("PlainClone(%q) error = %v", name, err)
	}
	g.Close()
	return Repository{Name: name}
}

func TestValidateRepoName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple", "myrepo", false},
		{"valid with dots", "my.repo", false},
		{"valid with dashes", "my-repo", false},
		{"valid with underscores", "my_repo", false},
		{"valid with numbers", "repo123", false},
		{"valid starting with number", "123repo", false},
		{"empty name", "", true},
		{"starts with dot", ".repo", true},
		{"starts with dash", "-repo", true},
		{"contains space", "my repo", true},
		{"contains slash", "my/repo", true},
		{"contains special chars", "my@repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepoName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepoName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestSanitizeRepoPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean path", "myrepo", "myrepo"},
		{"leading slash", "/myrepo", "myrepo"},
		{"trailing slash", "myrepo/", "myrepo"},
		{"double dots", "my/../repo", "my//repo"},
		{"multiple slashes", "my//repo", "my//repo"},
		{"complex traversal", "../../../etc/passwd", "etc/passwd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeRepoPath(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeRepoPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildRepoPath(t *testing.T) {
	tests := []struct {
		name       string
		base       string
		components []string
		want       string
	}{
		{"single component", "/repos", []string{"myrepo"}, "/repos/myrepo"},
		{"multiple components", "/repos", []string{"org", "myrepo"}, "/repos/org/myrepo"},
		{"sanitizes input", "/repos", []string{"../../../etc"}, "/repos/etc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRepoPath(tt.base, tt.components...)
			if got != tt.want {
				t.Errorf("BuildRepoPath(%q, %v) = %q, want %q", tt.base, tt.components, got, tt.want)
			}
		})
	}
}

func TestService_Create(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	ownerID := uuid.New()

	t.Run("creates valid bare repo", func(t *testing.T) {
		input := CreateInput{
			Name:        "test-repo",
			Description: "A test repository",
			IsPrivate:   true,
			OwnerID:     ownerID,
			DefaultRef:  "refs/heads/main",
		}

		repo, err := svc.Create(ctx, input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if repo.Name != input.Name {
			t.Errorf("Create() repo.Name = %q, want %q", repo.Name, input.Name)
		}
		if repo.Description != input.Description {
			t.Errorf("Create() repo.Description = %q, want %q", repo.Description, input.Description)
		}
		if repo.IsPrivate != input.IsPrivate {
			t.Errorf("Create() repo.IsPrivate = %v, want %v", repo.IsPrivate, input.IsPrivate)
		}
		if repo.OwnerID != ownerID {
			t.Errorf("Create() repo.OwnerID = %v, want %v", repo.OwnerID, ownerID)
		}

		if !svc.Exists(repo) {
			t.Error("Create() repository should exist on disk")
		}
	})

	t.Run("rejects invalid name", func(t *testing.T) {
		input := CreateInput{
			Name:    "../bad-repo",
			OwnerID: ownerID,
		}

		_, err := svc.Create(ctx, input)
		if err == nil {
			t.Fatal("Create() expected error for invalid name, got nil")
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		input := CreateInput{
			Name:    "",
			OwnerID: ownerID,
		}

		_, err := svc.Create(ctx, input)
		if err == nil {
			t.Fatal("Create() expected error for empty name, got nil")
		}
	})

	t.Run("creates repo with default branch", func(t *testing.T) {
		input := CreateInput{
			Name:       "branch-test",
			OwnerID:    ownerID,
			DefaultRef: "refs/heads/develop",
		}

		repo, err := svc.Create(ctx, input)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		path := svc.RepoPath(repo)
		headFile := filepath.Join(path, "HEAD")
		data, err := os.ReadFile(headFile)
		if err != nil {
			t.Fatalf("failed to read HEAD file: %v", err)
		}

		if string(data) != "ref: refs/heads/develop\n" {
			t.Errorf("HEAD = %q, want %q", string(data), "ref: refs/heads/develop\n")
		}
	})
}

func TestService_GetByName(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	ownerID := uuid.New()

	t.Run("finds existing repo", func(t *testing.T) {
		createInput := CreateInput{
			Name:    "findable-repo",
			OwnerID: ownerID,
		}
		_, err := svc.Create(ctx, createInput)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		repo, err := svc.GetByName(ctx, ownerID, "findable-repo")
		if err != nil {
			t.Fatalf("GetByName() error = %v", err)
		}
		if repo.Name != "findable-repo" {
			t.Errorf("GetByName() repo.Name = %q, want %q", repo.Name, "findable-repo")
		}
	})

	t.Run("returns not found for missing repo", func(t *testing.T) {
		_, err := svc.GetByName(ctx, ownerID, "nonexistent")
		if err == nil {
			t.Fatal("GetByName() expected error for nonexistent repo, got nil")
		}
	})
}

func TestService_GetRefs(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	ownerID := uuid.New()

	t.Run("returns refs for populated repo", func(t *testing.T) {
		repo := newClonedRepo(t, svc, "refs-repo-populated")

		refs, err := svc.GetRefs(ctx, repo)
		if err != nil {
			t.Fatalf("GetRefs() error = %v", err)
		}

		if len(refs) == 0 {
			t.Error("GetRefs() expected refs, got empty list")
		}

		found := false
		for _, ref := range refs {
			if ref.Name == "HEAD" {
				found = true
				break
			}
		}
		if !found {
			t.Error("GetRefs() expected HEAD ref, not found")
		}
	})

	t.Run("returns refs for empty repo", func(t *testing.T) {
		createInput := CreateInput{
			Name:    "empty-refs-repo",
			OwnerID: ownerID,
		}
		repo, err := svc.Create(ctx, createInput)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		refs, err := svc.GetRefs(ctx, repo)
		if err != nil {
			t.Fatalf("GetRefs() error = %v", err)
		}

		if len(refs) == 0 {
			t.Error("GetRefs() expected at least HEAD ref, got empty list")
		}
	})
}

func TestService_GetStats(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	t.Run("returns stats for cloned repo", func(t *testing.T) {
		repo := newClonedRepo(t, svc, "stats-repo-cloned")

		stats, err := svc.GetStats(ctx, repo)
		if err != nil {
			t.Fatalf("GetStats() error = %v", err)
		}

		if stats.CommitCount == 0 {
			t.Error("GetStats() expected commits, got 0")
		}
		if stats.BranchCount == 0 {
			t.Error("GetStats() expected branches, got 0")
		}
	})
}

func TestService_Exists(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	ownerID := uuid.New()

	t.Run("returns true for existing repo", func(t *testing.T) {
		createInput := CreateInput{
			Name:    "exists-repo",
			OwnerID: ownerID,
		}
		repo, err := svc.Create(ctx, createInput)
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if !svc.Exists(repo) {
			t.Error("Exists() should return true for existing repo")
		}
	})

	t.Run("returns false for nonexistent repo", func(t *testing.T) {
		repo := Repository{Name: "does-not-exist"}
		if svc.Exists(repo) {
			t.Error("Exists() should return false for nonexistent repo")
		}
	})
}

func TestService_RepoPath(t *testing.T) {
	svc, dir := newTestService(t)

	repo := Repository{Name: "test-repo"}
	got := svc.RepoPath(repo)
	want := dir + "/test-repo"
	if got != want {
		t.Errorf("RepoPath() = %q, want %q", got, want)
	}
}

func TestSmartHTTPHandler_InfoRefs(t *testing.T) {
	svc, _ := newTestService(t)
	newClonedRepo(t, svc, "http-test-repo-cloned")

	handler := NewSmartHTTPHandler(svc)

	t.Run("GET info/refs for upload-pack", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/http-test-repo-cloned/info/refs?service=git-upload-pack", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("info/refs status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		ct := rec.Header().Get("Content-Type")
		wantCT := "application/x-git-upload-pack-advertisement"
		if ct != wantCT {
			t.Errorf("Content-Type = %q, want %q", ct, wantCT)
		}
	})

	t.Run("GET info/refs for receive-pack", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/http-test-repo-cloned/info/refs?service=git-receive-pack", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("info/refs status = %d, want %d, body: %s", rec.Code, http.StatusOK, rec.Body.String())
		}

		ct := rec.Header().Get("Content-Type")
		wantCT := "application/x-git-receive-pack-advertisement"
		if ct != wantCT {
			t.Errorf("Content-Type = %q, want %q", ct, wantCT)
		}
	})

	t.Run("GET info/refs for nonexistent repo", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/nonexistent/info/refs?service=git-upload-pack", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("info/refs status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("GET info/refs without service param", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/http-test-repo-cloned/info/refs", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("info/refs status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("GET info/refs with invalid service", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/http-test-repo-cloned/info/refs?service=invalid", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("info/refs status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestSmartHTTPHandler_UploadPack(t *testing.T) {
	svc, _ := newTestService(t)
	newClonedRepo(t, svc, "upload-pack-repo-cloned")

	handler := NewSmartHTTPHandler(svc)

	t.Run("POST git-upload-pack with invalid content-type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/upload-pack-repo-cloned/git-upload-pack", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("upload-pack status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("POST git-upload-pack for nonexistent repo", func(t *testing.T) {
		body := []byte("0000")
		req := httptest.NewRequest("POST", "/nonexistent/git-upload-pack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-git-upload-pack-request")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("upload-pack status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestSmartHTTPHandler_ReceivePack(t *testing.T) {
	svc, _ := newTestService(t)
	newClonedRepo(t, svc, "receive-pack-repo-cloned")

	handler := NewSmartHTTPHandler(svc)

	t.Run("POST git-receive-pack with invalid content-type", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/receive-pack-repo-cloned/git-receive-pack", bytes.NewReader([]byte{}))
		req.Header.Set("Content-Type", "text/plain")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("receive-pack status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("POST git-receive-pack for nonexistent repo", func(t *testing.T) {
		body := []byte("0000")
		req := httptest.NewRequest("POST", "/nonexistent/git-receive-pack", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("receive-pack status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})
}

func TestSmartHTTPHandler_Mount(t *testing.T) {
	svc, _ := newTestService(t)
	handler := NewSmartHTTPHandler(svc)

	mux := http.NewServeMux()
	handler.Mount(mux)

	if mux == nil {
		t.Error("Mount() should not panic")
	}
}

func TestSmartHTTPHandler_ServeMux(t *testing.T) {
	svc, _ := newTestService(t)
	handler := NewSmartHTTPHandler(svc)

	mux := handler.ServeMux()
	if mux == nil {
		t.Error("ServeMux() should return non-nil mux")
	}
}

func TestPktLineEncode(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "simple data",
			input:   []byte("hello"),
			wantErr: false,
		},
		{
			name:    "empty data",
			input:   []byte{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pktline.Encode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("pktline.Encode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				expected := fmt.Sprintf("%04x", len(tt.input)+4) + string(tt.input)
				if string(got) != expected {
					t.Errorf("pktline.Encode() = %q, want %q", string(got), expected)
				}
			}
		})
	}
}

func TestPktLineEncodeLine(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "simple data",
			input:   []byte("hello"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := pktline.EncodeLine(&buf, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("pktline.EncodeLine() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				expected := fmt.Sprintf("%04x", len(tt.input)+4) + string(tt.input)
				if buf.String() != expected {
					t.Errorf("pktline.EncodeLine() output = %q, want %q", buf.String(), expected)
				}
			}
		})
	}
}

func TestPktLineEncodeFlush(t *testing.T) {
	var buf bytes.Buffer
	err := pktline.EncodeFlush(&buf)
	if err != nil {
		t.Fatalf("pktline.EncodeFlush() error = %v", err)
	}
	if buf.String() != "0000" {
		t.Errorf("pktline.EncodeFlush() = %q, want %q", buf.String(), "0000")
	}
}

func TestPktLineDecode(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		want    pktline.Packet
		wantErr error
	}{
		{
			name:  "simple pkt-line",
			input: []byte("0009hello"),
			want: pktline.Packet{
				Type:     pktline.Data,
				Payload:  []byte("hello"),
				Consumed: 9,
			},
		},
		{
			name:  "flush packet",
			input: []byte("0000"),
			want: pktline.Packet{
				Type:     pktline.Flush,
				Consumed: 4,
			},
		},
		{
			name:  "delimiter packet",
			input: []byte("0001"),
			want: pktline.Packet{
				Type:     pktline.Delimiter,
				Consumed: 4,
			},
		},
		{
			name:  "response-end packet",
			input: []byte("0002"),
			want: pktline.Packet{
				Type:     pktline.ResponseEnd,
				Consumed: 4,
			},
		},
		{
			name:    "invalid length 3",
			input:   []byte("0003"),
			wantErr: pktline.ErrInvalidLength,
		},
		{
			name:    "empty packet",
			input:   []byte("0004"),
			wantErr: pktline.ErrEmptyPacket,
		},
		{
			name:    "incomplete data",
			input:   []byte("000ahel"),
			wantErr: pktline.ErrIncomplete,
		},
		{
			name:    "invalid hex",
			input:   []byte("zzzz"),
			wantErr: pktline.ErrInvalidCharacter,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pktline.Decode(tt.input)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("pktline.Decode() error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("pktline.Decode() unexpected error = %v", err)
			}
			if got.Type != tt.want.Type {
				t.Errorf("pktline.Decode() Type = %v, want %v", got.Type, tt.want.Type)
			}
			if !bytes.Equal(got.Payload, tt.want.Payload) {
				t.Errorf("pktline.Decode() Payload = %v, want %v", got.Payload, tt.want.Payload)
			}
			if got.Consumed != tt.want.Consumed {
				t.Errorf("pktline.Decode() Consumed = %d, want %d", got.Consumed, tt.want.Consumed)
			}
		})
	}
}

func TestPktLineStream(t *testing.T) {
	input := []byte("0009hello0000")
	stream := pktline.NewPacketIterator(bytes.NewReader(input))

	pkt, err := stream.Next()
	if err != nil {
		t.Fatalf("stream.Next() error = %v", err)
	}
	if pkt.Type != pktline.Data {
		t.Errorf("stream.Next() Type = %v, want %v", pkt.Type, pktline.Data)
	}
	if string(pkt.Payload) != "hello" {
		t.Errorf("stream.Next() Payload = %q, want %q", string(pkt.Payload), "hello")
	}

	pkt, err = stream.Next()
	if err != nil {
		t.Fatalf("stream.Next() flush error = %v", err)
	}
	if pkt.Type != pktline.Flush {
		t.Errorf("stream.Next() Type = %v, want %v", pkt.Type, pktline.Flush)
	}
}

func TestNopWriteCloser(t *testing.T) {
	var buf bytes.Buffer
	wc := &nopWriteCloser{&buf}

	if _, err := wc.Write([]byte("test")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := wc.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if buf.String() != "test" {
		t.Errorf("buffer = %q, want %q", buf.String(), "test")
	}
}

func TestService_UploadPack(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	repo := newClonedRepo(t, svc, "upload-svc-repo-cloned")

	t.Run("upload-pack advertisement", func(t *testing.T) {
		var buf bytes.Buffer
		req := UploadPackRequest{
			RepoPath:     repo.Name,
			Adverts:      true,
			StatelessRPC: true,
		}

		err := svc.UploadPack(ctx, req, &buf, http.NoBody)
		if err != nil {
			t.Fatalf("UploadPack() error = %v", err)
		}

		if buf.Len() == 0 {
			t.Error("UploadPack() expected output, got empty buffer")
		}
	})
}

func TestService_ReceivePack(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	repo := newClonedRepo(t, svc, "receive-svc-repo-cloned")

	t.Run("receive-pack with empty body", func(t *testing.T) {
		var buf bytes.Buffer
		req := ReceivePackRequest{
			RepoPath:     repo.Name,
			StatelessRPC: true,
		}

		err := svc.ReceivePack(ctx, req, &buf, bytes.NewReader([]byte("0000")))
		if err != nil {
			t.Fatalf("ReceivePack() error = %v", err)
		}
	})
}

func TestIntegration_CloneAndPush(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	repo := newClonedRepo(t, svc, "integration-repo-cloned")

	t.Run("verify refs after clone", func(t *testing.T) {
		refs, err := svc.GetRefs(ctx, repo)
		if err != nil {
			t.Fatalf("GetRefs() error = %v", err)
		}

		if len(refs) == 0 {
			t.Fatal("GetRefs() expected refs after clone")
		}

		foundMaster := false
		for _, ref := range refs {
			if ref.Name == "refs/heads/master" {
				foundMaster = true
				break
			}
		}
		if !foundMaster {
			t.Error("GetRefs() expected refs/heads/master, not found")
		}
	})

	t.Run("verify stats after clone", func(t *testing.T) {
		stats, err := svc.GetStats(ctx, repo)
		if err != nil {
			t.Fatalf("GetStats() error = %v", err)
		}

		if stats.CommitCount == 0 {
			t.Error("GetStats() expected commits after clone")
		}
	})
}
