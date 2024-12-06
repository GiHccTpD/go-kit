package lock

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLock struct {
	client    *redis.Client
	lockKey   string
	lockValue string
	expire    time.Duration
}

func NewRedisLock(client *redis.Client, key string, expire time.Duration) *RedisLock {
	return &RedisLock{
		client:    client,
		lockKey:   key,
		lockValue: fmt.Sprintf("%d", time.Now().UnixNano()), // 用唯一值标记锁持有者
		expire:    expire,
	}
}

// TryLock 尝试获取锁
func (l *RedisLock) TryLock(ctx context.Context) (bool, error) {
	result, err := l.client.SetNX(ctx, l.lockKey, l.lockValue, l.expire).Result()
	if err != nil {
		return false, err
	}
	return result, nil
}

// Unlock 释放锁
func (l *RedisLock) Unlock(ctx context.Context) error {
	// Lua 脚本：确保释放的是自己持有的锁
	luaScript := `
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end`
	_, err := l.client.Eval(ctx, luaScript, []string{l.lockKey}, l.lockValue).Result()
	return err
}
