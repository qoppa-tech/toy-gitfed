-- Token bucket rate limiter.
-- KEYS[1] = bucket key (e.g. "rl:ip:1.2.3.4")
-- ARGV[1] = rate (tokens per second, as float string)
-- ARGV[2] = burst (max tokens, as integer string)
-- ARGV[3] = now (current unix timestamp as float string)
--
-- Returns: {allowed (0|1), remaining (int), retry_after (float seconds)}

local key = KEYS[1]
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local data = redis.call("HMGET", key, "tokens", "last")
local tokens = tonumber(data[1])
local last = tonumber(data[2])

if tokens == nil then
  tokens = burst
  last = now
end

local elapsed = math.max(0, now - last)
tokens = math.min(burst, tokens + elapsed * rate)
last = now

local allowed = 0
local retry_after = 0

if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  retry_after = (1 - tokens) / rate
end

redis.call("HMSET", key, "tokens", tokens, "last", last)

local ttl = math.ceil(burst / rate) + 1
redis.call("EXPIRE", key, ttl)

return {allowed, math.floor(tokens), string.format("%.3f", retry_after)}
