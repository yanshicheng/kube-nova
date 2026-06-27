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

type DeleteInspectionGroupLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除巡检分组
func NewDeleteInspectionGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInspectionGroupLogic {
	return &DeleteInspectionGroupLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteInspectionGroupLogic) DeleteInspectionGroup(req *types.InspectionGroupDelRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.InspectionGroupDel(l.ctx, &pb.InspectionGroupDelReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		return "", err
	}

	return "OK", nil
}
