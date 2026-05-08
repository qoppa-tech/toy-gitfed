package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	githttp "github.com/qoppa-tech/gitfed/internal/api/http"
	"github.com/qoppa-tech/gitfed/internal/config"
	"github.com/qoppa-tech/gitfed/internal/database"
	"github.com/qoppa-tech/gitfed/internal/database/sqlc"
	gitmod "github.com/qoppa-tech/gitfed/internal/modules/git"
	"github.com/qoppa-tech/gitfed/internal/modules/organization"
	"github.com/qoppa-tech/gitfed/internal/modules/session"
	"github.com/qoppa-tech/gitfed/internal/modules/sso"
	"github.com/qoppa-tech/gitfed/internal/modules/user"
	"github.com/qoppa-tech/gitfed/internal/ratelimit"
	"github.com/qoppa-tech/gitfed/internal/store"
	"github.com/qoppa-tech/gitfed/pkg/logger"
)

func main() {
	cfg := config.Load()

	log := logger.New(cfg.Log)
	logger.SetDefault(log)

	if err := cfg.Validate(); err != nil {
		if verr, ok := err.(*config.ValidationError); ok {
			log.Error("config validation failed", "missing_vars", verr.MissingVars, "invalid_vars", verr.InvalidVars)
		} else {
			log.Error("config validation failed", "error", err)
		}
		os.Exit(1)
	}

	ctx := context.Background()

	// Infrastructure.
	dbPool, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		log.Fatal("database connection failed", "error", err)
	}

	queries := sqlc.New(dbPool)

	redisStore, err := store.NewRedisStore(cfg.Redis)
	if err != nil {
		log.Fatal("redis connection failed", "error", err)
	}

	// Rate limiting.
	limiter := ratelimit.NewLimiter(redisStore.Client())
	ipRateLimit := ratelimit.IPMiddleware(limiter.Allow, cfg.RateLimit.IPRate, cfg.RateLimit.IPBurst)
	extractUser := func(ctx context.Context) (string, bool) {
		uid, ok := githttp.UserIDFromContext(ctx)
		if !ok {
			return "", false
		}
		return uid.String(), true
	}
	userRateLimit := ratelimit.UserMiddleware(limiter.Allow, extractUser, cfg.RateLimit.UserRate, cfg.RateLimit.UserBurst)

	// Store adapters.
	userStore := user.NewStore(queries)
	sessionPGStore := session.NewPGStore(queries)
	sessionTokenStore := session.NewRedisTokenStore(redisStore)
	ssoStore := sso.NewPGStore(queries)
	ssoStateStore := sso.NewRedisStateStore(redisStore)
	orgStore := organization.NewStore(queries)
	repoStore := gitmod.NewStore(queries)

	// Domain services.
	userSvc := user.NewService(userStore)
	sessionSvc := session.NewService(sessionPGStore, sessionTokenStore)
	ssoSvc := sso.NewService(ssoStore, ssoStateStore, cfg.Google)
	totpSvc := session.NewTOTPService(redisStore, cfg.TOTPIssuer)
	orgSvc := organization.NewService(orgStore)
	gitSvc := gitmod.NewService(cfg.ReposDir)

	srv := githttp.NewServer(githttp.Config{
		ReposDir:       cfg.ReposDir,
		Address:        cfg.HTTPAddr,
		AppVersion:     cfg.AppVersion,
		RepoStore:      repoStore,
		GitService:     gitSvc,
		UserService:    userSvc,
		SessionService: sessionSvc,
		SSOService:     ssoSvc,
		TOTPService:    totpSvc,
		OrgService:     orgSvc,
		Secure:         cfg.SecureCookies,
		IPRateLimit:    ipRateLimit,
		UserRateLimit:  userRateLimit,
		Logger:         log,
		DBHealth: func(ctx context.Context) error {
			return dbPool.Ping(ctx)
		},
		RedisHealth: func(ctx context.Context) error {
			return redisStore.Client().Ping(ctx).Err()
		},
		HealthcheckTimeout: cfg.HealthcheckTimeout,
	})

	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: srv,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "address", cfg.HTTPAddr, "version", cfg.AppVersion)
		errCh <- httpServer.ListenAndServe()
	}()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server failed", "error", err)
		}
		return
	case <-sigCtx.Done():
		log.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	}
	if err := redisStore.Close(); err != nil {
		log.Error("redis close failed", "error", err)
	}
	dbPool.Close()

	if err := <-errCh; err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("server exit error", "error", err)
	}
}
