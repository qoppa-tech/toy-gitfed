package did

import (
	"testing"

	apperrors "github.com/qoppa-tech/toy-gitfed/pkg/errors"
)

func TestParseUserDID(t *testing.T) {
	raw := "did:gitfed:user:alice@example.com"
	d, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse user did: %v", err)
	}

	if d.Method != MethodGitFed {
		t.Fatalf("method = %q, want %q", d.Method, MethodGitFed)
	}
	if d.PrincipalType != PrincipalTypeUser {
		t.Fatalf("principal = %q, want %q", d.PrincipalType, PrincipalTypeUser)
	}
	if d.Identifier != "alice" {
		t.Fatalf("identifier = %q, want %q", d.Identifier, "alice")
	}
	if d.Host != "example.com" {
		t.Fatalf("host = %q, want %q", d.Host, "example.com")
	}
	if got := d.String(); got != raw {
		t.Fatalf("string() = %q, want %q", got, raw)
	}
}

func TestParseServerDID(t *testing.T) {
	raw := "did:gitfed:server:Forge.EXAMPLE.com"
	d, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse server did: %v", err)
	}

	if d.PrincipalType != PrincipalTypeServer {
		t.Fatalf("principal = %q, want %q", d.PrincipalType, PrincipalTypeServer)
	}
	if d.Host != "forge.example.com" {
		t.Fatalf("host = %q, want %q", d.Host, "forge.example.com")
	}
	if got := d.String(); got != "did:gitfed:server:forge.example.com" {
		t.Fatalf("string() = %q", got)
	}
}

func TestParseRejectsUnsupportedMethod(t *testing.T) {
	_, err := Parse("did:web:user:alice@example.com")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected invalid_argument code, got: %v", err)
	}
}

func TestParseRejectsInvalidPrincipalFormat(t *testing.T) {
	_, err := Parse("did:gitfed:user:alice")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !apperrors.IsCode(err, apperrors.CodeInvalidArgument) {
		t.Fatalf("expected invalid_argument code, got: %v", err)
	}
}
