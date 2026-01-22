package distlock

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	defaultTTL = 30 * time.Second
)

type Lock struct {
	redis  *redis.Client
	key    string
	value  string
	ttl    time.Duration
	cancel context.CancelFunc
}

func NewLock(rdb *redis.Client, key string, ttl time.Duration) *Lock {
	if ttl == 0 {
		ttl = defaultTTL
	}
	return &Lock{
		redis: rdb,
		key:   key,
		value: uuid.NewString(),
		ttl:   ttl,
	}
}

// TryLock 尝试获取锁
func (l *Lock) TryLock(ctx context.Context) (bool, error) {
	ok, err := l.redis.SetNX(ctx, l.key, l.value, l.ttl).Result()
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	// 启动 watchdog 自动续期
	l.startWatchdog()
	return true, nil
}

func (l *Lock) startWatchdog() {
	ctx, cancel := context.WithCancel(context.Background())
	l.cancel = cancel

	go func() {
		ticker := time.NewTicker(l.ttl / 3)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// 续期脚本：仅自己可以续期
				script := redis.NewScript(`
					if redis.call("GET", KEYS[1]) == ARGV[1] then
						return redis.call("PEXPIRE", KEYS[1], ARGV[2])
					else
						return 0
					end
				`)
				_, _ = script.Run(ctx, l.redis, []string{l.key}, l.value, int(l.ttl.Milliseconds())).Result()
			}
		}
	}()
}

func (l *Lock) Unlock(ctx context.Context) error {
	if l.cancel != nil {
		l.cancel()
	}

	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)

	_, err := script.Run(ctx, l.redis, []string{l.key}, l.value).Result()
	return err
}
