package config

import (
	"github.com/qoppa-tech/gitfed/internal/database"
	"github.com/qoppa-tech/gitfed/internal/modules/sso"
	"github.com/qoppa-tech/gitfed/internal/store"
	"github.com/qoppa-tech/gitfed/pkg/env"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

type RateLimitConfig struct {
	IPRate    int
	IPBurst   int
	UserRate  int
	UserBurst int
}

type Config struct {
	Database  database.Config
	Redis     store.RedisConfig
	Google    sso.GoogleConfig
	RateLimit RateLimitConfig
	Log       logger.Config

	HTTPAddr      string
	TOTPIssuer    string
	SecureCookies bool
}

func Load() Config {
	return Config{
		Database: database.Config{
			Host:     env.Or("DB_HOST", "localhost"),
			Port:     env.Int("DB_PORT", 5432),
			User:     env.Or("DB_USER", "postgres"),
			Password: env.Or("DB_PASSWORD", "postgres"),
			DBName:   env.Or("DB_NAME", "gitfed"),
			SSLMode:  env.Or("DB_SSLMODE", "disable"),
		},
		Redis: store.RedisConfig{
			Host:     env.Or("REDIS_HOST", "localhost"),
			Port:     env.Int("REDIS_PORT", 6379),
			Password: env.Or("REDIS_PASSWORD", ""),
			DB:       env.Int("REDIS_DB", 0),
		},
		Google: sso.GoogleConfig{
			ClientID:     env.Or("GOOGLE_CLIENT_ID", ""),
			ClientSecret: env.Or("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  env.Or("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
		},
		RateLimit: RateLimitConfig{
			IPRate:    env.Int("RATE_LIMIT_IP_RATE", 100),
			IPBurst:   env.Int("RATE_LIMIT_IP_BURST", 20),
			UserRate:  env.Int("RATE_LIMIT_USER_RATE", 200),
			UserBurst: env.Int("RATE_LIMIT_USER_BURST", 40),
		},
		Log: logger.Config{
			Env:   env.Or("ENV", "DEV"),
			Level: env.Or("LOG_LEVEL", "info"),
		},
		HTTPAddr:      env.Or("HTTP_ADDR", "0.0.0.0:8080"),
		TOTPIssuer:    env.Or("TOTP_ISSUER", "gitfed"),
		SecureCookies: env.Bool("SECURE_COOKIES", false),
	}
}
