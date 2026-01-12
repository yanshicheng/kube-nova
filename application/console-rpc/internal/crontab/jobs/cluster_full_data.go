package jobs

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
)

// 集群全量数据同步

type ClusterFullDataSyncJob struct {
	*BaseJob
}

func NewClusterFullDataSyncJob(svcCtx *svc.ServiceContext) *ClusterFullDataSyncJob {
	return &ClusterFullDataSyncJob{
		BaseJob: NewBaseJob(svcCtx,
			"集群全量数据同步",
			"0 0 2 * * *", // 修改这里：每天 02:00:00 执行
			WithAllowConcurrent(false),
			WithTimeout(60*time.Minute),
		),
	}
}
func (c *ClusterFullDataSyncJob) Execute(ctx context.Context) error {
	c.Infof("开始执行集群全量数据同步任务.....")
	_, err := c.svcCtx.ManagerRpc.ClusterSyncAll(ctx, &managerservice.ClusterSyncAllReq{
		Operator: "SystemCronjob",
	})
	if err != nil {
		c.Errorf("集群全量数据同步任务执行失败: %v", err)
		return err
	}
	c.Infof("集群全量数据同步任务执行成功")
	return nil
}
