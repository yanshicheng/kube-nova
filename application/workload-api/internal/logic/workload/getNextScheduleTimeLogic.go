package workload

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNextScheduleTimeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 CronJob 下次执行时间
func NewGetNextScheduleTimeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNextScheduleTimeLogic {
	return &GetNextScheduleTimeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNextScheduleTimeLogic) GetNextScheduleTime(req *types.DefaultIdRequest) (resp *types.NextScheduleTimeResponse, err error) {
	// 获取集群客户端和资源详情
	client, versionDetail, err := getResourceClusterClient(l.ctx, l.svcCtx, req.Id)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 调用 CronJob operator 获取下次执行时间
	nextScheduleInfo, err := client.CronJob().GetNextScheduleTimeInfo(versionDetail.Namespace, versionDetail.ResourceName)
	if err != nil {
		l.Errorf("获取 CronJob 下次执行时间失败: %v", err)
		return nil, fmt.Errorf("获取下次执行时间失败")
	}

	// 构建响应
	resp = &types.NextScheduleTimeResponse{
		Schedule:    nextScheduleInfo.Schedule,
		Timezone:    nextScheduleInfo.Timezone,
		IsSuspended: nextScheduleInfo.IsSuspended,
		CurrentTime: nextScheduleInfo.CurrentTime.UnixMilli(), // 转换为毫秒时间戳
	}

	// 如果有下次执行时间，转换为毫秒时间戳
	if nextScheduleInfo.NextScheduleTime != nil {
		nextTimeMillis := nextScheduleInfo.NextScheduleTime.UnixMilli()
		resp.NextScheduleTime = nextTimeMillis
	}

	return resp, nil
}
