package main

import (
	"context"
	"fmt"
	"os"

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
	reposDir := os.Getenv("REPOS_DIR")
	if reposDir == "" && len(os.Args) >= 2 {
		reposDir = os.Args[1]
	}
	if reposDir == "" {
		fmt.Fprintf(os.Stderr, "usage: set REPOS_DIR or pass <repos-dir> as argument\n")
		os.Exit(1)
	}
	cfg := config.Load()

	log := logger.New(cfg.Log)
	logger.SetDefault(log)

	ctx := context.Background()

	// Infrastructure.
	dbPool, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		log.Fatal("database connection failed", "error", err)
	}
	defer dbPool.Close()

	queries := sqlc.New(dbPool)

	redisStore, err := store.NewRedisStore(cfg.Redis)
	if err != nil {
		log.Fatal("redis connection failed", "error", err)
	}
	defer redisStore.Close()

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
	gitSvc := gitmod.NewService(reposDir)

	srv := githttp.NewServer(githttp.Config{
		ReposDir:       reposDir,
		Address:        cfg.HTTPAddr,
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
	})

	log.Fatal("server failed", "error", srv.Serve())
}
