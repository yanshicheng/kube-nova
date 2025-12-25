package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCronJobHistoryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetCronJobHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCronJobHistoryLogic {
	return &GetCronJobHistoryLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCronJobHistoryLogic) GetCronJobHistory(req *types.DefaultIdRequest) (resp *types.CronJobHistoryResponse, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	history, err := client.CronJob().GetJobHistory(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取 CronJob 历史失败: %v", err)
		return nil, fmt.Errorf("获取 CronJob 历史失败")
	}

	jobs := make([]types.CronJobHistoryItem, 0, len(history.Jobs))
	for _, job := range history.Jobs {
		jobs = append(jobs, types.CronJobHistoryItem{
			Name:              job.Name,
			Status:            job.Status,
			Completions:       job.Completions,
			Succeeded:         job.Succeeded,
			Active:            job.Active,
			Failed:            job.Failed,
			StartTime:         job.StartTime,
			CompletionTime:    job.CompletionTime,
			Duration:          job.Duration,
			CreationTimestamp: job.CreationTimestamp,
		})
	}

	resp = &types.CronJobHistoryResponse{
		Jobs: jobs,
	}

	return resp, nil
}
