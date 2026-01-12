package jobs

import (
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/crontab"
	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// JobRegistry 任务注册中心
type JobRegistry struct {
	svcCtx  *svc.ServiceContext
	manager *crontab.Manager
}

// NewJobRegistry 创建任务注册中心
func NewJobRegistry(svcCtx *svc.ServiceContext, manager *crontab.Manager) *JobRegistry {
	return &JobRegistry{
		svcCtx:  svcCtx,
		manager: manager,
	}
}

// RegisterAll 注册所有定时任务
func (r *JobRegistry) RegisterAll() int {
	jobs := r.getAllJobs()
	successCount := 0

	for _, job := range jobs {
		if err := r.manager.RegisterJob(job); err != nil {
			logx.Errorf("[Crontab] 注册任务失败, job=%s, error=%v", job.Name(), err)
			continue
		}
		successCount++
		logx.Infof("[Crontab] 注册任务成功, job=%s, spec=%s", job.Name(), job.Spec())
	}

	return successCount
}

// getAllJobs 获取所有需要注册的任务
func (r *JobRegistry) getAllJobs() []crontab.Job {
	return []crontab.Job{
		// 定时任务在此加载

		// 账单生成任务
		NewBillingGenerateJob(r.svcCtx),

		// 集群全量数据同步任务
		NewClusterFullDataSyncJob(r.svcCtx),
	}
}

// SetupCronJobs 设置并注册所有定时任务
func SetupCronJobs(svcCtx *svc.ServiceContext, manager *crontab.Manager) int {
	registry := NewJobRegistry(svcCtx, manager)
	return registry.RegisterAll()
}
