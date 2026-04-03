package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repo   Repository
	tokens TokenStore
}

func NewService(repo Repository, tokens TokenStore) *Service {
	return &Service{repo: repo, tokens: tokens}
}

// Create generates a new access + refresh token pair.
func (s *Service) Create(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	accessToken, err := generateToken(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate access token: %w", err)
	}

	refreshToken, err := generateToken(32)
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate refresh token: %w", err)
	}

	sessionID, err := uuid.NewV7()
	if err != nil {
		return TokenPair{}, fmt.Errorf("generate uuid: %w", err)
	}

	if err := s.repo.Create(ctx, sessionID, userID, refreshToken); err != nil {
		return TokenPair{}, fmt.Errorf("create session: %w", err)
	}

	uidStr := userID.String()

	if err := s.tokens.SetAccessToken(ctx, accessToken, uidStr, AccessTokenTTL); err != nil {
		return TokenPair{}, fmt.Errorf("set access token: %w", err)
	}

	if err := s.tokens.SetRefreshToken(ctx, refreshToken, uidStr, RefreshTokenTTL); err != nil {
		return TokenPair{}, fmt.Errorf("set refresh token: %w", err)
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// ValidateAccess checks an access token against the token store.
func (s *Service) ValidateAccess(ctx context.Context, accessToken string) (uuid.UUID, error) {
	uidStr, err := s.tokens.GetAccessToken(ctx, accessToken)
	if err != nil || uidStr == "" {
		return uuid.Nil, ErrInvalidAccessToken
	}
	return uuid.Parse(uidStr)
}

// Validate implements the auth middleware's token validation interface.
func (s *Service) Validate(ctx context.Context, token string) (uuid.UUID, error) {
	return s.ValidateAccess(ctx, token)
}

// Refresh validates a refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (string, error) {
	// Fast path: check token store (Redis).
	uidStr, err := s.tokens.GetRefreshToken(ctx, refreshToken)
	if err != nil || uidStr == "" {
		// Slow path: check PostgreSQL.
		userID, err := s.repo.GetUserIDByRefreshToken(ctx, refreshToken)
		if errors.Is(err, ErrSessionNotFound) {
			return "", ErrInvalidRefreshToken
		}
		if err != nil {
			return "", fmt.Errorf("query session: %w", err)
		}
		uidStr = userID.String()

		// Re-populate cache.
		_ = s.tokens.SetRefreshToken(ctx, refreshToken, uidStr, RefreshTokenTTL)
	}

	accessToken, err := generateToken(32)
	if err != nil {
		return "", fmt.Errorf("generate access token: %w", err)
	}

	if err := s.tokens.SetAccessToken(ctx, accessToken, uidStr, AccessTokenTTL); err != nil {
		return "", fmt.Errorf("set access token: %w", err)
	}
	return accessToken, nil
}

// Revoke deletes a session and invalidates the refresh token.
func (s *Service) Revoke(ctx context.Context, refreshToken string) error {
	if err := s.repo.DeleteByRefreshToken(ctx, refreshToken); err != nil {
		return err
	}
	_ = s.tokens.DeleteRefreshToken(ctx, refreshToken)
	return nil
}

// RevokeAll deletes all sessions for a user.
func (s *Service) RevokeAll(ctx context.Context, userID uuid.UUID) error {
	return s.repo.DeleteByUserID(ctx, userID)
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
