package testutil

import "os"

const (
	defaultPostgresTestHostPort = "5433"
	defaultRedisTestHostPort    = "6380"
)

func PostgresTestHostPort() string {
	if p := os.Getenv("POSTGRES_TEST_HOST_PORT"); p != "" {
		return p
	}
	return defaultPostgresTestHostPort
}

func RedisTestHostPort() string {
	if p := os.Getenv("REDIS_TEST_HOST_PORT"); p != "" {
		return p
	}
	return defaultRedisTestHostPort
}
