package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed rate_limit.lua
var rateLimitScript string

// Limiter implements token bucket rate limiting backed by a Redis Lua script.
type Limiter struct {
	client *redis.Client
	script *redis.Script
}

// NewLimiter creates a Limiter using the embedded Lua script.
func NewLimiter(client *redis.Client) *Limiter {
	return &Limiter{
		client: client,
		script: redis.NewScript(rateLimitScript),
	}
}

// Allow checks whether a request identified by key is permitted under the
// given rate (tokens/second) and burst (max tokens). It returns whether the
// request is allowed, the number of tokens remaining, the time to wait before
// retrying (zero if allowed), and any error from Redis.
func (l *Limiter) Allow(ctx context.Context, key string, rate float64, burst int) (bool, int, time.Duration, error) {
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := l.script.Run(ctx, l.client,
		[]string{key},
		fmt.Sprintf("%.6f", rate),
		strconv.Itoa(burst),
		fmt.Sprintf("%.6f", now),
	).Slice()
	if err != nil {
		return false, 0, 0, fmt.Errorf("ratelimit script: %w", err)
	}

	allowed := res[0].(int64) == 1
	remaining := int(res[1].(int64))
	retryAfterStr, _ := res[2].(string)
	retryAfter, _ := strconv.ParseFloat(retryAfterStr, 64)
	return allowed, remaining, time.Duration(retryAfter * float64(time.Second)), nil
}
