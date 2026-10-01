package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// incrScript increments and starts the window IN ONE STEP. Two commands (INCR, then PEXPIRE) leave
// a gap: a process dying between them leaves a counter with no expiry, which locks that address out
// of the operator area forever. The PTTL < 0 branch repairs such a key if one exists anyway (written
// by an older version, or by hand).
var incrScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
if n == 1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
local ttl = redis.call('PTTL', KEYS[1])
if ttl < 0 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
  ttl = tonumber(ARGV[1])
end
return {n, ttl}
`)

// RedisCounter is the production Counter. Same client shape as idem.RedisStore: one client from a
// DSN, or an injected one for a deployment that builds its own pool or topology.
type RedisCounter struct {
	rdb redis.UniversalClient
}

// NewRedisCounter opens the client from REDIS_DSN (config.Config.RedisDSN().Lo()).
//
// An empty DSN IS an error here, unlike the caller's choice idem leaves open: this counter guards a
// security threshold, and "no Redis" must surface as a refusal at wiring time, not as a limiter that
// was silently never built.
func NewRedisCounter(dsn string) (*RedisCounter, error) {
	if dsn == "" {
		return nil, errors.New("ratelimit: REDIS_DSN rỗng")
	}
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		// The DSN carries a password; it is never echoed into the error (rule 8).
		return nil, fmt.Errorf("ratelimit: REDIS_DSN không phân tích được: %w", err)
	}
	return &RedisCounter{rdb: redis.NewClient(opt)}, nil
}

// NewRedisCounterWith wraps an already-built client.
func NewRedisCounterWith(rdb redis.UniversalClient) *RedisCounter { return &RedisCounter{rdb: rdb} }

// Incr implements Counter.
func (r *RedisCounter) Incr(ctx context.Context, key string, window time.Duration) (int64, time.Duration, error) {
	res, err := incrScript.Run(ctx, r.rdb, []string{key}, window.Milliseconds()).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("ratelimit: EVALSHA: %w", err)
	}
	return parseIncrResult(res)
}

// Close releases the connection pool.
func (r *RedisCounter) Close() error { return r.rdb.Close() }

// parseIncrResult reads the script's {count, pttl}. Anything else is a contract fault between this
// code and the script, reported as an error — which the limiter turns into "not allowed".
func parseIncrResult(res any) (int64, time.Duration, error) {
	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return 0, 0, fmt.Errorf("ratelimit: script answered %T, want a pair", res)
	}
	n, ok1 := arr[0].(int64)
	ms, ok2 := arr[1].(int64)
	if !ok1 || !ok2 || n < 1 {
		return 0, 0, errors.New("ratelimit: script answered a malformed pair")
	}
	return n, time.Duration(ms) * time.Millisecond, nil
}
