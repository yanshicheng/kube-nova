package crontab

import (
	"context"
	"time"
)

// NoopLocker 空操作锁，用于单机模式
type NoopLocker struct {
	nodeID string
}

// NewNoopLocker 创建空操作锁
func NewNoopLocker(nodeID string) *NoopLocker {
	return &NoopLocker{
		nodeID: nodeID,
	}
}

// TryLock 总是返回成功
func (l *NoopLocker) TryLock(ctx context.Context, jobName string, ttl time.Duration) (bool, func(), error) {
	return true, func() {}, nil
}

// IsLocked 总是返回未锁定
func (l *NoopLocker) IsLocked(ctx context.Context, jobName string) (bool, error) {
	return false, nil
}

// NodeID 返回节点ID
func (l *NoopLocker) NodeID() string {
	return l.nodeID
}
