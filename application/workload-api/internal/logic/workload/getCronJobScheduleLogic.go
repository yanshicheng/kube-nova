package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCronJobScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCronJobScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCronJobScheduleLogic {
	return &GetCronJobScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCronJobScheduleLogic) GetCronJobSchedule(req *types.DefaultIdRequest) (resp *types.CronJobScheduleConfig, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	config, err := client.CronJob().GetScheduleConfig(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取 CronJob 调度配置失败: %v", err)
		return nil, fmt.Errorf("获取 CronJob 调度配置失败")
	}

	resp = &types.CronJobScheduleConfig{
		Schedule:                   config.Schedule,
		Timezone:                   config.Timezone,
		ConcurrencyPolicy:          config.ConcurrencyPolicy,
		Suspend:                    config.Suspend,
		StartingDeadlineSeconds:    config.StartingDeadlineSeconds,
		SuccessfulJobsHistoryLimit: config.SuccessfulJobsHistoryLimit,
		FailedJobsHistoryLimit:     config.FailedJobsHistoryLimit,
	}

	return resp, nil
}
