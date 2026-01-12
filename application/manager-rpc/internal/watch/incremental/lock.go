package incremental

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	lockKeyPrefix     = "incr:lock:"
	defaultLockExpire = 30
	minLockExpire     = 5
)

// RedisDistributedLock Redis 分布式锁实现
type RedisDistributedLock struct {
	client *redis.Redis
	nodeID string
}

// NewRedisDistributedLock 创建 Redis 分布式锁
func NewRedisDistributedLock(client *redis.Redis, nodeID string) *RedisDistributedLock {
	return &RedisDistributedLock{
		client: client,
		nodeID: nodeID,
	}
}

// TryLock 尝试获取分布式锁
func (l *RedisDistributedLock) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, func(), error) {
	lockKey := l.buildLockKey(key)
	expireSeconds := int(ttl.Seconds())
	if expireSeconds < minLockExpire {
		expireSeconds = defaultLockExpire
	}

	lock := redis.NewRedisLock(l.client, lockKey)
	lock.SetExpire(expireSeconds)

	acquired, err := lock.AcquireCtx(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("获取锁失败: %w", err)
	}

	if !acquired {
		return false, nil, nil
	}

	release := func() {
		releaseCtx := context.Background()
		if _, err := lock.ReleaseCtx(releaseCtx); err != nil {
			logx.Errorf("[DistributedLock] 释放锁失败, key=%s, error=%v", key, err)
		}
	}

	return true, release, nil
}

// IsLocked 检查是否被锁定
func (l *RedisDistributedLock) IsLocked(ctx context.Context, key string) (bool, error) {
	lockKey := l.buildLockKey(key)
	exists, err := l.client.ExistsCtx(ctx, lockKey)
	if err != nil {
		return false, fmt.Errorf("检查锁状态失败: %w", err)
	}
	return exists, nil
}

// NodeID 返回节点ID
func (l *RedisDistributedLock) NodeID() string {
	return l.nodeID
}

func (l *RedisDistributedLock) buildLockKey(key string) string {
	return lockKeyPrefix + key
}

// LockWithAutoRenew 带自动续期的分布式锁
type LockWithAutoRenew struct {
	client       *redis.Redis
	nodeID       string
	renewalRatio float64
}

// NewLockWithAutoRenew 创建带自动续期的分布式锁
func NewLockWithAutoRenew(client *redis.Redis, nodeID string) *LockWithAutoRenew {
	return &LockWithAutoRenew{
		client:       client,
		nodeID:       nodeID,
		renewalRatio: 0.5,
	}
}

// TryLock 尝试获取锁并启动自动续期
func (l *LockWithAutoRenew) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, func(), error) {
	lockKey := l.buildLockKey(key)
	expireSeconds := int(ttl.Seconds())
	if expireSeconds < minLockExpire {
		expireSeconds = defaultLockExpire
	}

	lock := redis.NewRedisLock(l.client, lockKey)
	lock.SetExpire(expireSeconds)

	acquired, err := lock.AcquireCtx(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("获取锁失败: %w", err)
	}

	if !acquired {
		return false, nil, nil
	}

	// 启动自动续期
	renewCtx, cancelRenew := context.WithCancel(context.Background())
	renewInterval := time.Duration(float64(ttl) * l.renewalRatio)
	var renewFailCount int32

	go func() {
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := l.client.ExpireCtx(renewCtx, lockKey, expireSeconds); err != nil {
					atomic.AddInt32(&renewFailCount, 1)
					if atomic.LoadInt32(&renewFailCount) >= 3 {
						logx.Errorf("[DistributedLock] 续期连续失败3次，停止续期, key=%s", key)
						return
					}
					continue
				}
				atomic.StoreInt32(&renewFailCount, 0)
			}
		}
	}()

	release := func() {
		cancelRenew()
		releaseCtx := context.Background()
		if _, err := lock.ReleaseCtx(releaseCtx); err != nil {
			logx.Errorf("[DistributedLock] 释放锁失败, key=%s, error=%v", key, err)
		}
	}

	return true, release, nil
}

// IsLocked 检查是否被锁定
func (l *LockWithAutoRenew) IsLocked(ctx context.Context, key string) (bool, error) {
	lockKey := l.buildLockKey(key)
	exists, err := l.client.ExistsCtx(ctx, lockKey)
	if err != nil {
		return false, fmt.Errorf("检查锁状态失败: %w", err)
	}
	return exists, nil
}

// NodeID 返回节点ID
func (l *LockWithAutoRenew) NodeID() string {
	return l.nodeID
}

func (l *LockWithAutoRenew) buildLockKey(key string) string {
	return lockKeyPrefix + key
}

// NoopLocker 空操作锁（单副本或测试环境使用）
type NoopLocker struct {
	nodeID string
}

// NewNoopLocker 创建空操作锁
func NewNoopLocker(nodeID string) *NoopLocker {
	return &NoopLocker{nodeID: nodeID}
}

// TryLock 总是返回成功
func (l *NoopLocker) TryLock(ctx context.Context, key string, ttl time.Duration) (bool, func(), error) {
	return true, func() {}, nil
}

// IsLocked 总是返回未锁定
func (l *NoopLocker) IsLocked(ctx context.Context, key string) (bool, error) {
	return false, nil
}

// NodeID 返回节点ID
func (l *NoopLocker) NodeID() string {
	return l.nodeID
}
