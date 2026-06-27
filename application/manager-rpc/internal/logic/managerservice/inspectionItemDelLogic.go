package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionItemDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionItemDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionItemDelLogic {
	return &InspectionItemDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionItemDelLogic) InspectionItemDel(in *pb.InspectionItemDelReq) (*pb.DeleteClusterResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("巡检项ID不能为空")
	}
	if err := l.svcCtx.OnecInspectionItemModel.DeleteSoft(l.ctx, in.Id); err != nil {
		return nil, errorx.Msg("删除巡检项失败")
	}

	return &pb.DeleteClusterResp{}, nil
}
