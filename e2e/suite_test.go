package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qoppa-tech/toy-gitfed/e2e/testutil"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type E2ESuite struct {
	suite.Suite

	repoRoot string
	reposDir string
	baseURL  string

	ctx    context.Context
	cancel context.CancelFunc

	pgPool *pgxpool.Pool
	redis  *redis.Client
	apiCmd *exec.Cmd
	client *http.Client
}

const opTimeout = 30 * time.Second

func newSuiteRootContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func withOperationTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, timeout)
}

func TestE2E(t *testing.T) {
	suite.Run(t, new(E2ESuite))
}

func (s *E2ESuite) SetupSuite() {
	s.ctx, s.cancel = newSuiteRootContext()
	s.T().Cleanup(func() {
		if s.cancel != nil {
			s.cancel()
		}
	})

	_, file, _, ok := runtime.Caller(0)
	s.Require().True(ok)
	s.repoRoot = filepath.Clean(filepath.Join(filepath.Dir(file), ".."))

	var err error
	s.reposDir = s.T().TempDir()

	err = testutil.ComposeUp(s.repoRoot)
	s.Require().NoError(err)
	s.T().Cleanup(func() {
		if err := testutil.ComposeDown(s.repoRoot); err != nil {
			s.T().Errorf("compose down: %v", err)
		}
	})

	pgDSN := fmt.Sprintf(
		"postgres://gitfed_test:gitfed_test@%s/gitfed_test?sslmode=disable",
		net.JoinHostPort("127.0.0.1", testutil.PostgresTestHostPort()),
	)
	s.pgPool = s.requirePostgres(pgDSN)
	s.T().Cleanup(func() {
		if s.pgPool != nil {
			s.pgPool.Close()
		}
	})

	s.redis = s.requireRedis(net.JoinHostPort("127.0.0.1", testutil.RedisTestHostPort()))
	s.T().Cleanup(func() {
		if s.redis != nil {
			if err := s.redis.Close(); err != nil {
				s.T().Errorf("close redis: %v", err)
			}
		}
	})

	applyCtx, applyCancel := withOperationTimeout(s.ctx, opTimeout)
	err = testutil.ApplySchema(applyCtx, s.pgPool, s.repoRoot)
	applyCancel()
	s.Require().NoError(err)

	var httpAddr string
	s.apiCmd, httpAddr, err = testutil.StartAPI(s.repoRoot, s.reposDir)
	s.Require().NoError(err)
	s.baseURL = "http://" + httpAddr
	s.T().Cleanup(func() {
		if s.apiCmd != nil {
			if err := testutil.StopAPI(s.apiCmd); err != nil {
				s.T().Errorf("stop api: %v", err)
			}
		}
	})

	jar, err := cookiejar.New(nil)
	s.Require().NoError(err)
	s.client = &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}
}

func (s *E2ESuite) TearDownSuite() {}

func (s *E2ESuite) TestHarnessBooted() {
	s.Require().NotNil(s.client)
	s.Require().True(strings.HasPrefix(s.baseURL, "http://127.0.0.1:"))
	s.Require().NotNil(s.pgPool)
	s.Require().NotNil(s.redis)
}

func (s *E2ESuite) requirePostgres(dsn string) *pgxpool.Pool {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		opCtx, cancel := withOperationTimeout(s.ctx, 2*time.Second)
		pool, err := testutil.NewPGPool(opCtx, dsn)
		cancel()
		if err == nil {
			return pool
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	s.Require().NoError(fmt.Errorf("postgres not ready: %w", lastErr))
	return nil
}

func (s *E2ESuite) requireRedis(addr string) *redis.Client {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		opCtx, cancel := withOperationTimeout(s.ctx, 2*time.Second)
		client, err := testutil.NewRedisClient(opCtx, addr)
		cancel()
		if err == nil {
			return client
		}
		lastErr = err
		time.Sleep(500 * time.Millisecond)
	}
	s.Require().NoError(fmt.Errorf("redis not ready: %w", lastErr))
	return nil
}
