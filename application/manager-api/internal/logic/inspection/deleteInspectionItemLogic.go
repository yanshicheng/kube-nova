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

type DeleteInspectionItemLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除巡检项
func NewDeleteInspectionItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInspectionItemLogic {
	return &DeleteInspectionItemLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteInspectionItemLogic) DeleteInspectionItem(req *types.InspectionItemDelRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.InspectionItemDel(l.ctx, &pb.InspectionItemDelReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		return "", err
	}

	return "删除巡检项成功", nil
}
