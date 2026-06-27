// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package inspection

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type FinishInspectionRecordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 手动结束运行中巡检记录
func NewFinishInspectionRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FinishInspectionRecordLogic {
	return &FinishInspectionRecordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *FinishInspectionRecordLogic) FinishInspectionRecord(req *types.InspectionRecordFinishRequest) (resp *types.InspectionRecordFinishResponse, err error) {
	rpcResp, err := l.svcCtx.ManagerRpc.InspectionRecordFinish(l.ctx, &pb.InspectionRecordFinishReq{
		RecordId: req.RecordId,
		Message:  req.Message,
		Operator: currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	resp = &types.InspectionRecordFinishResponse{}
	if record := convertRecord(rpcResp.Record); record != nil {
		resp.Record = *record
	}

	return
}
