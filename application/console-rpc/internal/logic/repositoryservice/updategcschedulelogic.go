package repositoryservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGCScheduleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGCScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGCScheduleLogic {
	return &UpdateGCScheduleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGCScheduleLogic) UpdateGCSchedule(in *pb.UpdateGCScheduleReq) (*pb.UpdateGCScheduleResp, error) {
	client, err := l.svcCtx.HarborManager.Get(in.RegistryUuid)
	if err != nil {
		return nil, errorx.Msg("获取仓库客户端失败")
	}

	err = client.GC().UpdateSchedule(in.Schedule, in.DeleteUntagged)
	if err != nil {
		return nil, errorx.Msg("更新GC调度配置失败")
	}

	return &pb.UpdateGCScheduleResp{
		Message: "GC调度配置更新成功",
	}, nil
}
