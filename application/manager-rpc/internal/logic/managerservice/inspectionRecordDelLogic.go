package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/inspection"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionRecordDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionRecordDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionRecordDelLogic {
	return &InspectionRecordDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionRecordDelLogic) InspectionRecordDel(in *pb.InspectionRecordDelReq) (*pb.DeleteClusterResp, error) {
	if err := inspection.DeleteRecordHard(l.ctx, l.svcCtx, in.RecordId, in.Operator); err != nil {
		return nil, errorx.Msg(err.Error())
	}

	return &pb.DeleteClusterResp{}, nil
}
