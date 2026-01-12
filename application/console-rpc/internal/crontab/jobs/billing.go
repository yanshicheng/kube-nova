package jobs

import (
	"context"
	"time"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
)

// 每日凌晨生成账单

type BillingGenerateJob struct {
	*BaseJob
}

func NewBillingGenerateJob(svcCtx *svc.ServiceContext) *BillingGenerateJob {
	return &BillingGenerateJob{
		BaseJob: NewBaseJob(
			svcCtx,
			"账单生成任务",
			"0 0 0 * * *",                // 修改这里：每天 00:00:00 执行
			WithTimeout(120*time.Minute), // 超时时间
			WithRetry(3, 5),              // 重试次数
			WithAllowConcurrent(false),   // 不允许并发
		),
	}
}

func (b *BillingGenerateJob) Execute(ctx context.Context) error {
	b.Infof("开始执行账单生成任务.....")
	_, err := b.svcCtx.ManagerRpc.OnecBillingStatementGenerateAll(ctx, &managerservice.OnecBillingStatementGenerateAllReq{
		UserName: "SystemCronjob",
	})
	if err != nil {
		b.Errorf("账单生成任务执行失败: %v", err)
		return err
	}
	b.Infof("账单生成任务执行成功")
	return nil
}
