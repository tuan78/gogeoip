package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache is a simple get/set cache interface.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool)
	Set(ctx context.Context, key string, val string, ttl time.Duration)
}

// NoopCache is a Cache implementation that does nothing.
type NoopCache struct{}

// Get always returns a cache miss.
func (NoopCache) Get(_ context.Context, _ string) (string, bool) { return "", false }

// Set is a no-op.
func (NoopCache) Set(_ context.Context, _ string, _ string, _ time.Duration) {}

// RedisCache is a Cache implementation backed by Redis.
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a RedisCache. Returns an error if the connection fails.
func NewRedisCache(addr, password string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       0,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &RedisCache{client: client}, nil
}

// Close closes the underlying Redis connection.
func (r *RedisCache) Close() error { return r.client.Close() }

// Get retrieves a value from Redis. Returns (value, true) on hit or ("", false) on miss.
func (r *RedisCache) Get(ctx context.Context, key string) (string, bool) {
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return val, true
}

// Set stores a value in Redis with the given TTL.
func (r *RedisCache) Set(ctx context.Context, key string, val string, ttl time.Duration) {
	r.client.Set(ctx, key, val, ttl) //nolint:errcheck
}
