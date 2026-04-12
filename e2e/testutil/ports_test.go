package testutil

import "testing"

func TestPostgresTestHostPort(t *testing.T) {
	t.Parallel()

	t.Setenv("POSTGRES_TEST_HOST_PORT", "")
	if got := PostgresTestHostPort(); got != "5433" {
		t.Fatalf("default postgres port = %q, want %q", got, "5433")
	}

	t.Setenv("POSTGRES_TEST_HOST_PORT", "15433")
	if got := PostgresTestHostPort(); got != "15433" {
		t.Fatalf("override postgres port = %q, want %q", got, "15433")
	}
}

func TestRedisTestHostPort(t *testing.T) {
	t.Parallel()

	t.Setenv("REDIS_TEST_HOST_PORT", "")
	if got := RedisTestHostPort(); got != "6380" {
		t.Fatalf("default redis port = %q, want %q", got, "6380")
	}

	t.Setenv("REDIS_TEST_HOST_PORT", "16380")
	if got := RedisTestHostPort(); got != "16380" {
		t.Fatalf("override redis port = %q, want %q", got, "16380")
	}
}
