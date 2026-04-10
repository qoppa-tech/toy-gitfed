package sso

import (
	"testing"

	"golang.org/x/oauth2"
)

func TestNewService_GoogleOAuthConfig(t *testing.T) {
	svc := NewService(nil, nil, GoogleConfig{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
		RedirectURL:  "http://localhost/callback",
	})

	if svc.googleOAuth == nil {
		t.Fatal("googleOAuth config should not be nil")
	}
	if svc.googleOAuth.ClientID != "test-id" {
		t.Errorf("ClientID = %q, want %q", svc.googleOAuth.ClientID, "test-id")
	}
	if svc.googleOAuth.ClientSecret != "test-secret" {
		t.Errorf("ClientSecret = %q, want %q", svc.googleOAuth.ClientSecret, "test-secret")
	}
	if svc.googleOAuth.RedirectURL != "http://localhost/callback" {
		t.Errorf("RedirectURL = %q, want %q", svc.googleOAuth.RedirectURL, "http://localhost/callback")
	}

	expectedScopes := []string{"openid", "email", "profile"}
	if len(svc.googleOAuth.Scopes) != len(expectedScopes) {
		t.Fatalf("Scopes length = %d, want %d", len(svc.googleOAuth.Scopes), len(expectedScopes))
	}
	for i, s := range expectedScopes {
		if svc.googleOAuth.Scopes[i] != s {
			t.Errorf("Scope[%d] = %q, want %q", i, svc.googleOAuth.Scopes[i], s)
		}
	}
}

func TestGoogleConfig_Endpoint(t *testing.T) {
	svc := NewService(nil, nil, GoogleConfig{
		ClientID:     "id",
		ClientSecret: "secret",
		RedirectURL:  "http://localhost/cb",
	})

	if svc.googleOAuth.Endpoint == (oauth2.Endpoint{}) {
		t.Error("Google endpoint should be configured")
	}
}
