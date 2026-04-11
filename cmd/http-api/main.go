package main

import (
	"context"
	"fmt"
	"log"
	"os"

	githttp "github.com/qoppa-tech/toy-gitfed/internal/api/http"
	"github.com/qoppa-tech/toy-gitfed/internal/config"
	"github.com/qoppa-tech/toy-gitfed/internal/database"
	"github.com/qoppa-tech/toy-gitfed/internal/database/sqlc"
	gitmod "github.com/qoppa-tech/toy-gitfed/internal/modules/git"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/organization"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/session"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/sso"
	"github.com/qoppa-tech/toy-gitfed/internal/modules/user"
	"github.com/qoppa-tech/toy-gitfed/internal/ratelimit"
	"github.com/qoppa-tech/toy-gitfed/internal/store"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <repos-dir>\n", os.Args[0])
		os.Exit(1)
	}

	reposDir := os.Args[1]
	cfg := config.Load()
	ctx := context.Background()

	// Infrastructure.
	dbPool, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer dbPool.Close()

	queries := sqlc.New(dbPool)

	redisStore, err := store.NewRedisStore(cfg.Redis)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisStore.Close()

	// Rate limiting.
	limiter := ratelimit.NewLimiter(redisStore.Client(), "scripts/rate_limit.lua")
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
		GitService:     gitSvc,
		UserService:    userSvc,
		SessionService: sessionSvc,
		SSOService:     ssoSvc,
		TOTPService:    totpSvc,
		OrgService:     orgSvc,
		Secure:         cfg.SecureCookies,
		IPRateLimit:    ipRateLimit,
		UserRateLimit:  userRateLimit,
	})

	log.Fatal(srv.Serve())
}
