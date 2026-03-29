package http

import (
	"testing"
)

func TestParseGitURL_InfoRefsUploadPack(t *testing.T) {
	p := parseGitURL("/myrepo.git/info/refs", "service=git-upload-pack")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Repo != "myrepo.git" {
		t.Errorf("repo = %q, want %q", p.Repo, "myrepo.git")
	}
	if p.Service != infoRefs {
		t.Errorf("service = %v, want infoRefs", p.Service)
	}
	if p.Query != "service=git-upload-pack" {
		t.Errorf("query = %q, want %q", p.Query, "service=git-upload-pack")
	}
}

func TestParseGitURL_InfoRefsReceivePack(t *testing.T) {
	p := parseGitURL("/myrepo.git/info/refs", "service=git-receive-pack")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Repo != "myrepo.git" {
		t.Errorf("repo = %q, want %q", p.Repo, "myrepo.git")
	}
	if p.Service != infoRefs {
		t.Errorf("service = %v, want infoRefs", p.Service)
	}
}

func TestParseGitURL_UploadPackPost(t *testing.T) {
	p := parseGitURL("/myrepo.git/git-upload-pack", "")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Repo != "myrepo.git" {
		t.Errorf("repo = %q, want %q", p.Repo, "myrepo.git")
	}
	if p.Service != uploadPack {
		t.Errorf("service = %v, want uploadPack", p.Service)
	}
}

func TestParseGitURL_ReceivePackPost(t *testing.T) {
	p := parseGitURL("/myrepo.git/git-receive-pack", "")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Repo != "myrepo.git" {
		t.Errorf("repo = %q, want %q", p.Repo, "myrepo.git")
	}
	if p.Service != receivePack {
		t.Errorf("service = %v, want receivePack", p.Service)
	}
}

func TestParseGitURL_NestedRepoPath(t *testing.T) {
	p := parseGitURL("/org/team/repo.git/git-upload-pack", "")
	if p == nil {
		t.Fatal("expected non-nil")
	}
	if p.Repo != "org/team/repo.git" {
		t.Errorf("repo = %q, want %q", p.Repo, "org/team/repo.git")
	}
}

func TestParseGitURL_RejectsUnknownPaths(t *testing.T) {
	cases := []struct {
		path, query string
	}{
		{"/", ""},
		{"/info/refs", ""},
		{"/repo/objects/info/packs", ""},
		{"/repo/info/refs", ""},  // no service= query
	}
	for _, tc := range cases {
		if p := parseGitURL(tc.path, tc.query); p != nil {
			t.Errorf("parseGitURL(%q, %q) = %+v, want nil", tc.path, tc.query, p)
		}
	}
}

func TestFindHeaderSep_CRLF(t *testing.T) {
	data := []byte("Content-Type: text/plain\r\nStatus: 200 OK\r\n\r\nbody")
	_, bodyStart, ok := findHeaderSep(data)
	if !ok {
		t.Fatal("expected separator")
	}
	if string(data[bodyStart:]) != "body" {
		t.Errorf("body = %q, want %q", string(data[bodyStart:]), "body")
	}
}

func TestFindHeaderSep_LF(t *testing.T) {
	data := []byte("Content-Type: text/plain\n\nbody")
	_, bodyStart, ok := findHeaderSep(data)
	if !ok {
		t.Fatal("expected separator")
	}
	if string(data[bodyStart:]) != "body" {
		t.Errorf("body = %q, want %q", string(data[bodyStart:]), "body")
	}
}

func TestFindHeaderSep_None(t *testing.T) {
	_, _, ok := findHeaderSep([]byte("no separator here"))
	if ok {
		t.Error("expected no separator")
	}
}
