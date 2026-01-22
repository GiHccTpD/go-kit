package distlock

import (
	"context"

	"github.com/redis/go-redis/v9"
)

type TokenGenerator struct {
	rdb      *redis.Client
	tokenKey string
}

func NewTokenGenerator(rdb *redis.Client, key string) *TokenGenerator {
	return &TokenGenerator{
		rdb:      rdb,
		tokenKey: key,
	}
}

func (t *TokenGenerator) NextToken(ctx context.Context) (int64, error) {
	return t.rdb.Incr(ctx, t.tokenKey).Result()
}
