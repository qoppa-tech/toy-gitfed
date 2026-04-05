package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	githttp "github.com/qoppa-tech/toy-gitfed/internal/api/http"
	"github.com/qoppa-tech/toy-gitfed/internal/database"
	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
	"github.com/qoppa-tech/toy-gitfed/internal/server/modules/sso"
	"github.com/qoppa-tech/toy-gitfed/internal/store"
)

var DefaultPort = 8080

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <repos-dir>\n", os.Args[0])
		os.Exit(1)
	}

	reposDir := os.Args[1]
	ctx := context.Background()

	// PostgreSQL.
	dbPool, err := database.Connect(ctx, database.Config{
		Host:     envOr("DB_HOST", "localhost"),
		Port:     envInt("DB_PORT", 5432),
		User:     envOr("DB_USER", "postgres"),
		Password: envOr("DB_PASSWORD", "postgres"),
		DBName:   envOr("DB_NAME", "gitfed"),
		SSLMode:  envOr("DB_SSLMODE", "disable"),
	})
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer dbPool.Close()

	queries := sqlc.New(dbPool)

	// Redis.
	redis, err := store.NewRedisStore(store.RedisConfig{
		Host:     envOr("REDIS_HOST", "localhost"),
		Port:     envInt("REDIS_PORT", 6379),
		Password: envOr("REDIS_PASSWORD", ""),
		DB:       envInt("REDIS_DB", 0),
	})
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redis.Close()

	addr := fmt.Sprintf("0.0.0.0:%d", DefaultPort)
	srv := githttp.NewServer(githttp.Config{
		ReposDir: reposDir,
		Address:  addr,
		Queries:  queries,
		Redis:    redis,
		GoogleOAuth: sso.GoogleConfig{
			ClientID:     envOr("GOOGLE_CLIENT_ID", ""),
			ClientSecret: envOr("GOOGLE_CLIENT_SECRET", ""),
			RedirectURL:  envOr("GOOGLE_REDIRECT_URL", "http://localhost:8080/auth/google/callback"),
		},
		TOTPIssuer: envOr("TOTP_ISSUER", "gitfed"),
	})

	log.Fatal(srv.Serve())
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}
