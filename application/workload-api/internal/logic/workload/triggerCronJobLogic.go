package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type TriggerCronJobLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewTriggerCronJobLogic(ctx context.Context, svcCtx *svc.ServiceContext) *TriggerCronJobLogic {
	return &TriggerCronJobLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *TriggerCronJobLogic) TriggerCronJob(req *types.DefaultIdRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	triggerReq := &k8sTypes.TriggerCronJobRequest{
		CronJobName: versionDetail.ResourceName,
		Namespace:   versionDetail.Namespace,
		JobName:     "",
	}

	job, err := client.CronJob().TriggerJob(triggerReq)
	if err != nil {
		l.Errorf("手动触发 CronJob 失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "手动触发CronJob",
			fmt.Sprintf("CronJob %s/%s 手动触发失败: %v", versionDetail.Namespace, versionDetail.ResourceName, err), 2)
		return "", fmt.Errorf("手动触发 CronJob 失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "手动触发CronJob",
		fmt.Sprintf("CronJob %s/%s 手动触发成功, 创建Job: %s", versionDetail.Namespace, versionDetail.ResourceName, job.Name), 1)
	return fmt.Sprintf("成功触发 CronJob，创建 Job: %s", job.Name), nil
}
