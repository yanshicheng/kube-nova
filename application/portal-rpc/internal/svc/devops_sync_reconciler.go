package svc

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

func (s *ServiceContext) startDevopsSyncReconciler() {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.reconcileDevopsSyncTasks()
			default:
				if s.AggregatorService != nil {
					select {
					case <-s.AggregatorService.Context().Done():
						return
					default:
					}
				}
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()
}

func (s *ServiceContext) reconcileDevopsSyncTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	tasks, err := s.ProjectPlatformSyncTaskModel.FindPending(ctx, 20)
	if err != nil {
		logx.WithContext(ctx).Errorf("扫描 DevOps 补偿同步任务失败: %v", err)
		return
	}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := s.ExecuteDevopsSyncTask(ctx, task); err != nil {
			logx.WithContext(ctx).Errorf("执行 DevOps 补偿同步任务失败: portalProjectUuid=%s action=%s retry=%d err=%v", task.PortalProjectUuid, task.Action, task.RetryCount, err)
		}
	}
}
