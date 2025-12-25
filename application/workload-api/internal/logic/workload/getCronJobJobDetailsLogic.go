package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetCronJobJobDetailsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询 CronJob Job 详情
func NewGetCronJobJobDetailsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetCronJobJobDetailsLogic {
	return &GetCronJobJobDetailsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetCronJobJobDetailsLogic) GetCronJobJobDetails(req *types.GetDefaultJobNameRequest) (resp string, err error) {
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}
	jobOperator := client.Job()
	job, err := jobOperator.GetDescribe(versionDetail.Namespace, req.JobName)
	if err != nil {
		l.Errorf("获取 Job 详情失败: %v", err)
		return "", fmt.Errorf("获取 Job 详情失败")
	}
	return job, err
}
