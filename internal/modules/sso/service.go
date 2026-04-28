package sso

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type Service struct {
	repo        Repository
	states      StateStore
	googleOAuth *oauth2.Config
}

func NewService(repo Repository, states StateStore, gcfg GoogleConfig) *Service {
	return &Service{
		repo:   repo,
		states: states,
		googleOAuth: &oauth2.Config{
			ClientID:     gcfg.ClientID,
			ClientSecret: gcfg.ClientSecret,
			RedirectURL:  gcfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// GoogleAuthURL generates the OAuth2 authorization URL with a random state param.
func (s *Service) GoogleAuthURL(ctx context.Context) (string, error) {
	state := uuid.New().String()
	if err := s.states.SetOAuthState(ctx, state, 10*time.Minute); err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}
	return s.googleOAuth.AuthCodeURL(state), nil
}

// GoogleCallback exchanges the authorization code and fetches user info.
func (s *Service) GoogleCallback(ctx context.Context, state, code string) (*GoogleUserInfo, error) {
	valid, err := s.states.GetOAuthState(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("check oauth state: %w", err)
	}
	if !valid {
		return nil, ErrInvalidState
	}
	if err := s.states.DeleteOAuthState(ctx, state); err != nil {
		logger.FromContext(ctx).Warn("oauth state delete failed", "step", "oauth_state_cleanup", "error", err)
	}

	token, err := s.googleOAuth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oauth exchange: %w", err)
	}

	client := s.googleOAuth.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		return nil, fmt.Errorf("fetch userinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo response %d: %s", resp.StatusCode, body)
	}

	var info GoogleUserInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}
	return &info, nil
}

// FindOrCreateSSO looks up an existing SSO record or creates a new one.
func (s *Service) FindOrCreateSSO(ctx context.Context, userID uuid.UUID, provider Provider, name, username string) (Record, error) {
	existing, err := s.repo.GetByProviderAndUsername(ctx, provider, username)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, ErrProviderNotFound) {
		return Record{}, fmt.Errorf("lookup sso: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Record{}, fmt.Errorf("generate uuid: %w", err)
	}

	record, err := s.repo.Create(ctx, Record{
		ID:       id,
		UserID:   userID,
		Name:     name,
		Provider: provider,
		Username: username,
	})
	if err != nil {
		return Record{}, fmt.Errorf("create sso: %w", err)
	}
	return record, nil
}

// GetByUserID returns all SSO providers linked to a user.
func (s *Service) GetByUserID(ctx context.Context, userID uuid.UUID) ([]Record, error) {
	return s.repo.GetByUserID(ctx, userID)
}
