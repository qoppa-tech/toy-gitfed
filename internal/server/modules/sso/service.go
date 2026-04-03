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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
	"github.com/qoppa-tech/toy-gitfed/internal/store"
)

var (
	ErrProviderNotFound = errors.New("sso provider record not found")
	ErrInvalidState     = errors.New("invalid oauth state")
)

type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Service struct {
	queries     *sqlc.Queries
	redis       *store.RedisStore
	googleOAuth *oauth2.Config
}

func NewService(queries *sqlc.Queries, redis *store.RedisStore, gcfg GoogleConfig) *Service {
	return &Service{
		queries: queries,
		redis:   redis,
		googleOAuth: &oauth2.Config{
			ClientID:     gcfg.ClientID,
			ClientSecret: gcfg.ClientSecret,
			RedirectURL:  gcfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
	}
}

// GoogleAuthURL generates the OAuth2 authorization URL with a random state param stored in Redis.
func (s *Service) GoogleAuthURL(ctx context.Context) (string, error) {
	state := uuid.New().String()
	if err := s.redis.SetOAuthState(ctx, state, 10*time.Minute); err != nil {
		return "", fmt.Errorf("store oauth state: %w", err)
	}
	return s.googleOAuth.AuthCodeURL(state), nil
}

type GoogleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// GoogleCallback exchanges the authorization code and fetches user info.
func (s *Service) GoogleCallback(ctx context.Context, state, code string) (*GoogleUserInfo, error) {
	valid, err := s.redis.GetOAuthState(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("check oauth state: %w", err)
	}
	if !valid {
		return nil, ErrInvalidState
	}
	_ = s.redis.DeleteOAuthState(ctx, state)

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

// FindOrCreateSSO looks up an existing SSO record or creates one for the user.
func (s *Service) FindOrCreateSSO(ctx context.Context, userID pgtype.UUID, provider, name, username string) (sqlc.SsoProvider, error) {
	existing, err := s.queries.GetSSOByProviderAndUsername(ctx, sqlc.GetSSOByProviderAndUsernameParams{
		Provider: provider,
		Username: pgtype.Text{String: username, Valid: true},
	})
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return sqlc.SsoProvider{}, fmt.Errorf("lookup sso: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return sqlc.SsoProvider{}, fmt.Errorf("generate uuid: %w", err)
	}

	record, err := s.queries.CreateSSO(ctx, sqlc.CreateSSOParams{
		SsoID:    pgtype.UUID{Bytes: id, Valid: true},
		UserID:   userID,
		Name:     name,
		Provider: provider,
		Username: pgtype.Text{String: username, Valid: true},
	})
	if err != nil {
		return sqlc.SsoProvider{}, fmt.Errorf("create sso: %w", err)
	}
	return record, nil
}

// GetByUserID returns all SSO providers linked to a user.
func (s *Service) GetByUserID(ctx context.Context, userID pgtype.UUID) ([]sqlc.SsoProvider, error) {
	return s.queries.GetSSOByUserID(ctx, userID)
}
