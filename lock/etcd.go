package lock

//
//import (
//	"context"
//	"fmt"
//	"sync"
//	"time"
//
//	"go.etcd.io/etcd/client/v3"
//	"go.etcd.io/etcd/client/v3/concurrency"
//)
//
//var (
//	etcdClient *clientv3.Client
//	once       sync.Once
//)
//
//// InitializeEtcdClient 初始化全局 etcd 客户端，支持用户名和密码
//func InitializeEtcdClient(endpoints []string, username, password string) error {
//	var err error
//	once.Do(func() {
//		etcdClient, err = clientv3.New(clientv3.Config{
//			Endpoints:   endpoints,
//			DialTimeout: 5 * time.Second,
//			Username:    username,
//			Password:    password,
//		})
//	})
//	return err
//}
//
//// DistributedLock 分布式锁结构
//type DistributedLock struct {
//	client  *clientv3.Client
//	session *concurrency.Session
//	mutex   *concurrency.Mutex
//}
//
//// NewDistributedLock 创建一个新的分布式锁
//func NewDistributedLock(lockKey string, ttl int) (*DistributedLock, error) {
//	// 创建会话，支持自定义 TTL
//	session, err := concurrency.NewSession(etcdClient, concurrency.WithTTL(ttl))
//	if err != nil {
//		return nil, fmt.Errorf("failed to create etcd session: %w", err)
//	}
//
//	// 创建锁
//	mutex := concurrency.NewMutex(session, lockKey)
//
//	return &DistributedLock{
//		client:  etcdClient,
//		session: session,
//		mutex:   mutex,
//	}, nil
//}
//
//// Lock 加锁
//func (dl *DistributedLock) Lock() error {
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	if err := dl.mutex.Lock(ctx); err != nil {
//		return fmt.Errorf("failed to acquire lock: %w", err)
//	}
//	return nil
//}
//
//// Unlock 解锁
//func (dl *DistributedLock) Unlock() error {
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	if err := dl.mutex.Unlock(ctx); err != nil {
//		return fmt.Errorf("failed to release lock: %w", err)
//	}
//
//	// 关闭会话，释放资源
//	if err := dl.session.Close(); err != nil {
//		return fmt.Errorf("failed to close session: %w", err)
//	}
//
//	return nil
//}
