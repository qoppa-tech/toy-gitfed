package store

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

func (c RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(cfg RedisConfig) (*RedisStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Close() error {
	return s.client.Close()
}

// Access token operations (short-lived, Redis-only)

func accessKey(token string) string {
	return "access:" + token
}

func (s *RedisStore) SetAccessToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, accessKey(token), userID, ttl).Err()
}

func (s *RedisStore) GetAccessToken(ctx context.Context, token string) (string, error) {
	return s.client.Get(ctx, accessKey(token)).Result()
}

func (s *RedisStore) DeleteAccessToken(ctx context.Context, token string) error {
	return s.client.Del(ctx, accessKey(token)).Err()
}

// Refresh token operations (long-lived, also stored in PostgreSQL)

func refreshKey(token string) string {
	return "refresh:" + token
}

func (s *RedisStore) SetRefreshToken(ctx context.Context, token string, userID string, ttl time.Duration) error {
	return s.client.Set(ctx, refreshKey(token), userID, ttl).Err()
}

func (s *RedisStore) GetRefreshToken(ctx context.Context, token string) (string, error) {
	return s.client.Get(ctx, refreshKey(token)).Result()
}

func (s *RedisStore) DeleteRefreshToken(ctx context.Context, token string) error {
	return s.client.Del(ctx, refreshKey(token)).Err()
}

// TOTP operations

func totpKey(userID string) string {
	return "totp:" + userID
}

func (s *RedisStore) SetTOTPSecret(ctx context.Context, userID string, secret string, ttl time.Duration) error {
	return s.client.Set(ctx, totpKey(userID), secret, ttl).Err()
}

func (s *RedisStore) GetTOTPSecret(ctx context.Context, userID string) (string, error) {
	return s.client.Get(ctx, totpKey(userID)).Result()
}

func (s *RedisStore) DeleteTOTPSecret(ctx context.Context, userID string) error {
	return s.client.Del(ctx, totpKey(userID)).Err()
}

// OAuth state operations

func oauthStateKey(state string) string {
	return "oauth_state:" + state
}

func (s *RedisStore) SetOAuthState(ctx context.Context, state string, ttl time.Duration) error {
	return s.client.Set(ctx, oauthStateKey(state), "1", ttl).Err()
}

func (s *RedisStore) GetOAuthState(ctx context.Context, state string) (bool, error) {
	err := s.client.Get(ctx, oauthStateKey(state)).Err()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisStore) DeleteOAuthState(ctx context.Context, state string) error {
	return s.client.Del(ctx, oauthStateKey(state)).Err()
}
