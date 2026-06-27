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

type DeleteInspectionRecordLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除巡检报告
func NewDeleteInspectionRecordLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInspectionRecordLogic {
	return &DeleteInspectionRecordLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteInspectionRecordLogic) DeleteInspectionRecord(req *types.InspectionRecordDelRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.InspectionRecordDel(l.ctx, &pb.InspectionRecordDelReq{
		RecordId: req.RecordId,
		Operator: currentUsername(l.ctx),
	})
	if err != nil {
		return "", err
	}

	return "删除巡检报告成功", nil
}
