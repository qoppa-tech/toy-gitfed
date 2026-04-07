package ratelimit

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter implements token bucket rate limiting backed by a Redis Lua script.
type Limiter struct {
	client *redis.Client
	script *redis.Script
}

// NewLimiter creates a Limiter by loading the Lua script at scriptPath.
func NewLimiter(client *redis.Client, scriptPath string) *Limiter {
	src, err := os.ReadFile(scriptPath)
	if err != nil {
		panic(fmt.Sprintf("ratelimit: read script %s: %v", scriptPath, err))
	}
	return &Limiter{
		client: client,
		script: redis.NewScript(string(src)),
	}
}

// Allow checks whether a request identified by key is permitted under the
// given rate (tokens/second) and burst (max tokens). It returns whether the
// request is allowed, the time to wait before retrying (zero if allowed), and
// any error from Redis.
func (l *Limiter) Allow(ctx context.Context, key string, rate float64, burst int) (bool, time.Duration, error) {
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := l.script.Run(ctx, l.client,
		[]string{key},
		fmt.Sprintf("%.6f", rate),
		strconv.Itoa(burst),
		fmt.Sprintf("%.6f", now),
	).Slice()
	if err != nil {
		return false, 0, fmt.Errorf("ratelimit script: %w", err)
	}

	allowed := res[0].(int64) == 1
	retryAfterStr, _ := res[2].(string)
	retryAfter, _ := strconv.ParseFloat(retryAfterStr, 64)
	return allowed, time.Duration(retryAfter * float64(time.Second)), nil
}
