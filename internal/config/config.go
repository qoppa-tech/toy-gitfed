package config

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

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

	HTTPAddr           string
	ReposDir           string
	AppVersion         string
	TOTPIssuer         string
	SecureCookies      bool
	ShutdownTimeout    time.Duration
	HealthcheckTimeout time.Duration
	SeedAdminName      string
	SeedAdminUsername  string
	SeedAdminEmail     string
	SeedAdminPassword  string
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
		HTTPAddr:           env.Or("HTTP_ADDR", "0.0.0.0:8080"),
		ReposDir:           env.Or("REPOS_DIR", ""),
		AppVersion:         env.Or("APP_VERSION", "dev"),
		TOTPIssuer:         env.Or("TOTP_ISSUER", "gitfed"),
		SecureCookies:      env.Bool("SECURE_COOKIES", false),
		ShutdownTimeout:    parseDuration("SHUTDOWN_TIMEOUT", 15*time.Second),
		HealthcheckTimeout: parseDuration("HEALTHCHECK_TIMEOUT", 2*time.Second),
		SeedAdminName:      env.Or("SEED_ADMIN_NAME", ""),
		SeedAdminUsername:  env.Or("SEED_ADMIN_USERNAME", ""),
		SeedAdminEmail:     env.Or("SEED_ADMIN_EMAIL", ""),
		SeedAdminPassword:  env.Or("SEED_ADMIN_PASSWORD", ""),
	}
}

type ValidationError struct {
	MissingVars []string
	InvalidVars map[string]string
}

func (e *ValidationError) Error() string {
	return "configuration validation failed"
}

func (c Config) Validate() error {
	missing := make([]string, 0)
	invalid := make(map[string]string)

	envName := strings.ToLower(os.Getenv("ENV"))
	if envName == "" {
		envName = "dev"
	}

	required := []string{"REPOS_DIR"}
	if envName != "dev" {
		required = append(required,
			"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
			"REDIS_HOST", "REDIS_PORT",
			"HTTP_ADDR",
		)
	}

	for _, key := range required {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		invalid["DB_PORT"] = "must be between 1 and 65535"
	}
	if c.Redis.Port < 1 || c.Redis.Port > 65535 {
		invalid["REDIS_PORT"] = "must be between 1 and 65535"
	}
	if c.HTTPAddr == "" {
		invalid["HTTP_ADDR"] = "must not be empty"
	} else if _, err := net.ResolveTCPAddr("tcp", c.HTTPAddr); err != nil {
		invalid["HTTP_ADDR"] = fmt.Sprintf("invalid TCP address: %v", err)
	}
	if c.ReposDir == "" {
		invalid["REPOS_DIR"] = "must not be empty"
	}
	if c.ShutdownTimeout <= 0 {
		invalid["SHUTDOWN_TIMEOUT"] = "must be > 0"
	}
	if c.HealthcheckTimeout <= 0 {
		invalid["HEALTHCHECK_TIMEOUT"] = "must be > 0"
	}

	if len(missing) == 0 && len(invalid) == 0 {
		return nil
	}
	return &ValidationError{
		MissingVars: missing,
		InvalidVars: invalid,
	}
}

func (c Config) ValidateSeed() error {
	missing := make([]string, 0)
	for _, key := range []string{
		"SEED_ADMIN_NAME",
		"SEED_ADMIN_USERNAME",
		"SEED_ADMIN_EMAIL",
		"SEED_ADMIN_PASSWORD",
	} {
		if strings.TrimSpace(os.Getenv(key)) == "" {
			missing = append(missing, key)
		}
	}

	if len(missing) == 0 {
		return nil
	}
	return &ValidationError{
		MissingVars: missing,
		InvalidVars: map[string]string{},
	}
}

func parseDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
