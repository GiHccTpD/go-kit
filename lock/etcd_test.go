package lock

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func Test_Lock(t *testing.T) {
	// etcd 配置信息
	endpoints := []string{"localhost:2379"}
	username := ""
	password := ""

	// 初始化 etcd 客户端
	if err := InitializeEtcdClient(endpoints, username, password); err != nil {
		fmt.Println("Failed to initialize etcd client:", err)
		return
	}
	defer etcdClient.Close() // 程序结束时关闭连接

	var wg sync.WaitGroup
	maxGoroutines := 1000
	sem := make(chan struct{}, maxGoroutines)

	for i := 0; i < 20000; i++ {
		wg.Add(1)
		i := i

		go func() {
			defer wg.Done()

			// 使用信号量限制并发数
			sem <- struct{}{}
			defer func() { <-sem }()

			// 创建分布式锁
			lockKey := fmt.Sprintf("/distributed-lock/example-%d", i)
			lock, err := NewDistributedLock(lockKey, 10)
			if err != nil {
				fmt.Println("Failed to create lock:", err)
				return
			}

			// 加锁
			fmt.Println("Trying to acquire lock...")
			if err := lock.Lock(); err != nil {
				fmt.Println("Failed to acquire lock:", err)
				return
			}
			fmt.Println("Lock acquired!")

			// 模拟任务执行
			time.Sleep(1 * time.Second)

			// 解锁
			if err := lock.Unlock(); err != nil {
				fmt.Println("Failed to release lock:", err)
				return
			}
			fmt.Println("Lock released!")
		}()
	}

	wg.Wait()
}
