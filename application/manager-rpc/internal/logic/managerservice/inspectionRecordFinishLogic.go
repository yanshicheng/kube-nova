package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/inspection"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type InspectionRecordFinishLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewInspectionRecordFinishLogic(ctx context.Context, svcCtx *svc.ServiceContext) *InspectionRecordFinishLogic {
	return &InspectionRecordFinishLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *InspectionRecordFinishLogic) InspectionRecordFinish(in *pb.InspectionRecordFinishReq) (*pb.InspectionRecordFinishResp, error) {
	record, err := inspection.FinishRecordManually(l.ctx, l.svcCtx, in.RecordId, in.Message, in.Operator)
	if err != nil {
		return nil, errorx.Msg(err.Error())
	}

	return &pb.InspectionRecordFinishResp{Record: inspection.ToPBRecord(record)}, nil
}
