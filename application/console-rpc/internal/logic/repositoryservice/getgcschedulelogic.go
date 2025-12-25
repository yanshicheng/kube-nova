package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGCScheduleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGCScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGCScheduleLogic {
	return &GetGCScheduleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGCScheduleLogic) GetGCSchedule(in *pb.GetGCScheduleReq) (*pb.GetGCScheduleResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	schedule, err := client.GC().GetSchedule()
	if err != nil {
		return nil, errorx.Msg("获取GC调度配置失败")
	}

	scheduleStr := ""
	if schedule.Schedule != nil {
		scheduleStr = schedule.Schedule.Cron
	}

	return &pb.GetGCScheduleResp{
		Data: &pb.GCSchedule{
			Schedule:   scheduleStr,
			Parameters: schedule.Parameters,
		},
	}, nil
}
