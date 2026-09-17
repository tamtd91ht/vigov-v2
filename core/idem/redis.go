package idem

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisStore is the production Store.
//
// ADR 0010 allows Redis for cache and rate limiting and FORBIDS it as durable storage
// (kb/10-decisions/0010-data-infrastructure.md). This stays inside that boundary: the truth is
// the PostgreSQL row, the key is a 24-hour guard holding a status and a business code —
// never a response body, never personal data (rule 3).
type RedisStore struct {
	rdb redis.UniversalClient
}

// NewRedisStore opens the client from a DSN, e.g. redis://host:6379/0.
//
// An empty DSN is NOT an error here: it is the caller's decision. Local development with no
// Redis passes nil to Middleware and every route behaves per its declared CheDoHong. A
// service that refuses to start because a cache is absent is a service nobody can run.
func NewRedisStore(dsn string) (*RedisStore, error) {
	if dsn == "" {
		return nil, errors.New("idem: REDIS_DSN rỗng")
	}
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		// The DSN carries a password; it is never echoed into the error (rule 8).
		return nil, fmt.Errorf("idem: REDIS_DSN không phân tích được: %w", err)
	}
	return &RedisStore{rdb: redis.NewClient(opt)}, nil
}

// NewRedisStoreWith wraps an already-built client, for a deployment that configures its own
// pool, TLS or cluster topology.
func NewRedisStoreWith(rdb redis.UniversalClient) *RedisStore { return &RedisStore{rdb: rdb} }

func (s *RedisStore) Claim(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := s.rdb.SetNX(ctx, key, dauDangChay, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("idem: SET NX: %w", err)
	}
	return ok, nil
}

func (s *RedisStore) Get(ctx context.Context, key string) (string, error) {
	v, err := s.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil // absent is not a failure
	}
	if err != nil {
		return "", fmt.Errorf("idem: GET: %w", err)
	}
	return v, nil
}

func (s *RedisStore) Complete(ctx context.Context, key, value string, ttl time.Duration) error {
	if err := s.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("idem: SET: %w", err)
	}
	return nil
}

func (s *RedisStore) Release(ctx context.Context, key string) error {
	if err := s.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("idem: DEL: %w", err)
	}
	return nil
}

// Close releases the connection pool.
func (s *RedisStore) Close() error { return s.rdb.Close() }
