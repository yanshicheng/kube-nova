package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionTaskDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionTaskDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionTaskDelLogic {
	return &InspectionTaskDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionTaskDelLogic) InspectionTaskDel(in *pb.InspectionTaskDelReq) (*pb.DeleteClusterResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("任务ID不能为空")
	}
	if err := l.svcCtx.OnecInspectionTaskModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除巡检任务失败")
	}

	return &pb.DeleteClusterResp{}, nil
}
