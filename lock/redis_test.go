package lock

import (
	"context"
	"fmt"
	"github.com/redis/go-redis/v9"
	"sync"
	"testing"
	"time"
)

// 示例代码
func Test_TryLock(t *testing.T) {
	// 初始化 Redis 客户端
	client := redis.NewClient(&redis.Options{
		Addr:         "10.110.124.115:1502", // Redis 地址
		Password:     "Ccwork1024",          // 密码 (如果有的话)
		DB:           0,                     // 使用的数据库编号
		PoolSize:     1000,                  // 连接池大小，设置为 1000
		MinIdleConns: 100,                   // 最小空闲连接数
	})

	var wg = sync.WaitGroup{}

	for i := 0; i < 5000; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			key := fmt.Sprintf("%s:%d", "my_distributed_lock", i)
			// 创建锁实例
			lock := NewRedisLock(client, key, 10*time.Second)

			// 尝试获取锁
			ctx := context.Background()
			locked, err := lock.TryLock(ctx)
			if err != nil {
				fmt.Println("Error while trying to acquire lock:", err)
				return
			} else {
				// 释放锁
				defer lock.Unlock(ctx)
			}
			if !locked {
				fmt.Println("Lock is already held by someone else")
				return
			}

			fmt.Println("Lock acquired!", key)

			// 业务逻辑
			time.Sleep(5 * time.Second)

			//// 释放锁
			//err = lock.Unlock(ctx)
			//if err != nil {
			//	fmt.Println("Error while releasing lock:", err)
			//	return
			//}

			fmt.Println("Lock released!", key)
		}()
	}
	defer wg.Wait()
}
