package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisCache struct {
	Cli *redis.Client
	TTL time.Duration
}

func New(addr string, db int, ttlSeconds int) *RedisCache {
	return &RedisCache{
		Cli: redis.NewClient(&redis.Options{Addr: addr, DB: db}),
		TTL: time.Duration(ttlSeconds) * time.Second,
	}
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.Cli.Get(ctx, key).Result()
}

func (r *RedisCache) Set(ctx context.Context, key string, val string) error {
	return r.Cli.Set(ctx, key, val, r.TTL).Err()
}

func (r *RedisCache) Del(ctx context.Context, key string) error {
	return r.Cli.Del(ctx, key).Err()
}
