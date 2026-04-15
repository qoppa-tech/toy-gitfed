package ratelimit

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func testRedisClient(t *testing.T) *redis.Client {
	t.Helper()
	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() {
		client.FlushDB(context.Background())
		client.Close()
	})
	return client
}

func TestLimiter_Allow(t *testing.T) {
	client := testRedisClient(t)
	limiter := NewLimiter(client)

	ctx := context.Background()
	key := "rl:test:allow"

	// Burst of 3 should allow 3 requests immediately.
	for i := range 3 {
		allowed, _, _, err := limiter.Allow(ctx, key, 1.0, 3)
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	// 4th request should be denied.
	allowed, _, retryAfter, err := limiter.Allow(ctx, key, 1.0, 3)
	if err != nil {
		t.Fatalf("request 4: %v", err)
	}
	if allowed {
		t.Fatal("request 4 should be denied")
	}
	if retryAfter <= 0 {
		t.Errorf("retryAfter = %v, want > 0", retryAfter)
	}
}

func TestLimiter_Refill(t *testing.T) {
	client := testRedisClient(t)
	limiter := NewLimiter(client)

	ctx := context.Background()
	key := "rl:test:refill"

	// Drain the bucket (burst=1, rate=10/s).
	allowed, _, _, err := limiter.Allow(ctx, key, 10.0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("first request should be allowed")
	}

	// Should be denied now.
	allowed, _, _, err = limiter.Allow(ctx, key, 10.0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("second request should be denied")
	}

	// Wait for refill (100ms for 1 token at 10/s).
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again.
	allowed, _, _, err = limiter.Allow(ctx, key, 10.0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("third request should be allowed after refill")
	}
}

func TestLimiter_BurstExhaustion(t *testing.T) {
	client := testRedisClient(t)
	limiter := NewLimiter(client)

	ctx := context.Background()
	key := "rl:test:exhaustion"

	// With burst=5, first request should be allowed.
	allowed, _, _, err := limiter.Allow(ctx, key, 1.0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("should be allowed")
	}

	// Drain remaining 4.
	for range 4 {
		limiter.Allow(ctx, key, 1.0, 5)
	}

	// Next should be denied.
	allowed, _, retryAfter, err := limiter.Allow(ctx, key, 1.0, 5)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("should be denied after burst exhausted")
	}
	if retryAfter <= 0 {
		t.Error("retryAfter should be > 0")
	}
}
