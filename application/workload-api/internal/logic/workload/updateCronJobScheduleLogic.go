package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	k8sTypes "github.com/yanshicheng/kube-nova/common/k8smanager/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateCronJobScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateCronJobScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCronJobScheduleLogic {
	return &UpdateCronJobScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateCronJobScheduleLogic) UpdateCronJobSchedule(req *types.UpdateCronJobScheduleRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	updateReq := &k8sTypes.UpdateCronJobScheduleRequest{
		Name:                       versionDetail.ResourceName,
		Namespace:                  versionDetail.Namespace,
		Schedule:                   req.Schedule,
		Timezone:                   req.Timezone,
		ConcurrencyPolicy:          req.ConcurrencyPolicy,
		StartingDeadlineSeconds:    &req.StartingDeadlineSeconds,
		SuccessfulJobsHistoryLimit: &req.SuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     &req.FailedJobsHistoryLimit,
	}

	err = client.CronJob().UpdateScheduleConfig(updateReq)
	if err != nil {
		l.Errorf("修改 CronJob 调度配置失败: %v", err)
		recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
			fmt.Sprintf("CronJob %s/%s 修改调度配置失败, 目标调度表达式: %s, 错误: %v", versionDetail.Namespace, versionDetail.ResourceName, req.Schedule, err), 2)
		return "", fmt.Errorf("修改 CronJob 调度配置失败")
	}

	recordAuditLog(l.ctx, l.svcCtx, versionDetail, "修改调度配置",
		fmt.Sprintf("CronJob %s/%s 修改调度配置成功, 新调度表达式: %s, 并发策略: %s", versionDetail.Namespace, versionDetail.ResourceName, req.Schedule, req.ConcurrencyPolicy), 1)
	return "修改 CronJob 调度配置成功", nil
}
