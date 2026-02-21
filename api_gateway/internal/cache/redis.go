package cache

import (
	"common_library/logging"
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisCache struct {
	rdb    *redis.Client
	logger *logging.Logger
}

func NewRedisCache(rdb *redis.Client, logger *logging.Logger) *RedisCache {
	return &RedisCache{rdb: rdb, logger: logger}
}

func (r *RedisCache) Get(ctx context.Context, key string) ([]byte, bool) {
	val, err := r.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) || err != nil {
		return nil, false
	}
	return val, true
}

func (r *RedisCache) Set(ctx context.Context, key string, data []byte, ttl time.Duration) {
	if err := r.rdb.Set(ctx, key, data, ttl).Err(); err != nil {
		r.logger.Error(ctx, "failed to set cache", zap.String("key", key), zap.Error(err))
	}
}

func (r *RedisCache) Delete(ctx context.Context, key string) {
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		r.logger.Error(ctx, "failed to delete cache", zap.String("key", key), zap.Error(err))
	}
}
