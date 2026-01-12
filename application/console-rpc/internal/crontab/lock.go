package crontab

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

const (
	// lockKeyPrefix 分布式锁的 key 前缀
	lockKeyPrefix = "crontab:lock:"
	// defaultLockExpire 默认锁过期时间（秒）
	defaultLockExpire = 60
	// minLockExpire 最小锁过期时间（秒）
	minLockExpire = 1
)

// DistributedLocker 分布式锁接口
type DistributedLocker interface {
	// TryLock 获取分布式锁，jobName: 任务名称，ttl: 锁的过期时间
	TryLock(ctx context.Context, jobName string, ttl time.Duration) (acquired bool, release func(), err error)

	// IsLocked 检查任务是否被锁定
	IsLocked(ctx context.Context, jobName string) (bool, error)

	// NodeID 返回当前节点标识
	NodeID() string
}

// RedisDistributedLock 分布式锁实现
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
func (l *RedisDistributedLock) TryLock(ctx context.Context, jobName string, ttl time.Duration) (bool, func(), error) {
	key := l.buildLockKey(jobName)
	expireSeconds := int(ttl.Seconds())
	// 最小锁过期时间 如果传入的锁过期时间小于最小锁过期时间，则使用最小锁过期时间
	if expireSeconds < minLockExpire {
		expireSeconds = defaultLockExpire
	}

	lock := redis.NewRedisLock(l.client, key)
	// 必须在 Acquire 之前设置过期时间！
	lock.SetExpire(expireSeconds) // 设置锁的过期时间

	// 尝试获取锁
	acquired, err := lock.AcquireCtx(ctx)
	if err != nil {
		logx.WithContext(ctx).Errorf("Crontab 获取分布式锁失败, job=%s, node=%s, error=%v",
			jobName, l.nodeID, err)
		return false, nil, fmt.Errorf("获取锁失败: %w", err)
	}

	if !acquired {
		logx.WithContext(ctx).Infof("Crontab 任务已在其他节点执行, job=%s, node=%s",
			jobName, l.nodeID)
		return false, nil, nil
	}

	logx.WithContext(ctx).Infof("Crontab 成功获取分布式锁, job=%s, node=%s, ttl=%v",
		jobName, l.nodeID, ttl)

	// 返回释放锁的函数
	release := func() {
		releaseCtx := context.Background() // 使用新的 context，避免原 ctx 已取消
		if _, err := lock.ReleaseCtx(releaseCtx); err != nil {
			logx.Errorf("Crontab 释放分布式锁失败, job=%s, node=%s, error=%v",
				jobName, l.nodeID, err)
		} else {
			logx.Debugf("Crontab 成功释放分布式锁, job=%s, node=%s",
				jobName, l.nodeID)
		}
	}

	return true, release, nil
}

// IsLocked 检查任务是否被锁定
func (l *RedisDistributedLock) IsLocked(ctx context.Context, jobName string) (bool, error) {
	key := l.buildLockKey(jobName)
	exists, err := l.client.ExistsCtx(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查任务是否锁定失败: %w", err)
	}
	return exists, nil
}

// NodeID 返回当前节点ID
func (l *RedisDistributedLock) NodeID() string {
	return l.nodeID
}

// buildLockKey 构建锁的 key
func (l *RedisDistributedLock) buildLockKey(jobName string) string {
	return lockKeyPrefix + jobName
}

// LockWithAutoRenew 带自动续期的分布式锁
type LockWithAutoRenew struct {
	client       *redis.Redis
	nodeID       string
	renewalRatio float64 // 续期比例，例如 0.5 表示在锁过期前 50% 时续期
}

// NewLockWithAutoRenew 创建带自动续期的分布式锁
func NewLockWithAutoRenew(client *redis.Redis, nodeID string) *LockWithAutoRenew {
	return &LockWithAutoRenew{
		client:       client,
		nodeID:       nodeID,
		renewalRatio: 0.5, // 默认在过期前 50% 时续期
	}
}

// SetRenewalRatio 设置续期比例
func (l *LockWithAutoRenew) SetRenewalRatio(ratio float64) {
	if ratio > 0 && ratio < 1 {
		l.renewalRatio = ratio
	}
}

// TryLock 尝试获取锁并启动自动续期
func (l *LockWithAutoRenew) TryLock(ctx context.Context, jobName string, ttl time.Duration) (bool, func(), error) {
	key := l.buildLockKey(jobName)
	expireSeconds := int(ttl.Seconds())
	if expireSeconds < minLockExpire {
		expireSeconds = defaultLockExpire
	}

	lock := redis.NewRedisLock(l.client, key)
	lock.SetExpire(expireSeconds)

	acquired, err := lock.AcquireCtx(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire lock failed: %w", err)
	}

	if !acquired {
		return false, nil, nil
	}

	logx.WithContext(ctx).Infof("Crontab 成功获取分布式锁(自动续期), job=%s, node=%s, ttl=%v",
		jobName, l.nodeID, ttl)

	// 启动自动续期 goroutine
	renewCtx, cancelRenew := context.WithCancel(context.Background())
	renewInterval := time.Duration(float64(ttl) * l.renewalRatio)

	// 用于追踪续期失败次数
	var renewFailCount int32

	go func() {
		ticker := time.NewTicker(renewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				// 续期：重新设置过期时间
				if err := l.client.ExpireCtx(renewCtx, key, expireSeconds); err != nil {
					atomic.AddInt32(&renewFailCount, 1)
					logx.Errorf("Crontab 续期分布式锁失败, job=%s, failCount=%d, error=%v",
						jobName, atomic.LoadInt32(&renewFailCount), err)

					// 连续失败3次则停止续期（锁可能已丢失）
					if atomic.LoadInt32(&renewFailCount) >= 3 {
						logx.Errorf("Crontab 续期连续失败3次，停止续期, job=%s", jobName)
						return
					}
					continue
				}
				// 续期成功，重置失败计数
				atomic.StoreInt32(&renewFailCount, 0)
				logx.Debugf("Crontab 续期分布式锁成功, job=%s", jobName)
			}
		}
	}()

	release := func() {
		cancelRenew() // 停止续期
		releaseCtx := context.Background()
		if _, err := lock.ReleaseCtx(releaseCtx); err != nil {
			logx.Errorf("Crontab 释放分布式锁失败, job=%s, error=%v", jobName, err)
		} else {
			logx.Debugf("Crontab 成功释放分布式锁, job=%s", jobName)
		}
	}

	return true, release, nil
}

// IsLocked 检查任务是否被锁定
func (l *LockWithAutoRenew) IsLocked(ctx context.Context, jobName string) (bool, error) {
	key := l.buildLockKey(jobName)
	exists, err := l.client.ExistsCtx(ctx, key)
	if err != nil {
		return false, fmt.Errorf("检查任务是否锁定失败: %w", err)
	}
	return exists, nil
}

func (l *LockWithAutoRenew) NodeID() string {
	return l.nodeID
}

func (l *LockWithAutoRenew) buildLockKey(jobName string) string {
	return lockKeyPrefix + jobName
}
