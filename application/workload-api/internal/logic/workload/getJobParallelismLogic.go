package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetJobParallelismLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetJobParallelismLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetJobParallelismLogic {
	return &GetJobParallelismLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetJobParallelismLogic) GetJobParallelism(req *types.DefaultIdRequest) (resp *types.JobParallelismConfig, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	config, err := client.Job().GetParallelismConfig(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取 Job 并行度配置失败: %v", err)
		return nil, fmt.Errorf("获取 Job 并行度配置失败")
	}

	resp = &types.JobParallelismConfig{
		Parallelism:           config.Parallelism,
		Completions:           config.Completions,
		BackoffLimit:          config.BackoffLimit,
		ActiveDeadlineSeconds: config.ActiveDeadlineSeconds,
	}

	return resp, nil
}
