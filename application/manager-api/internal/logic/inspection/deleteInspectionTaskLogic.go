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

type DeleteInspectionTaskLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除巡检任务
func NewDeleteInspectionTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteInspectionTaskLogic {
	return &DeleteInspectionTaskLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteInspectionTaskLogic) DeleteInspectionTask(req *types.InspectionTaskDelRequest) (resp string, err error) {
	_, err = l.svcCtx.ManagerRpc.InspectionTaskDel(l.ctx, &pb.InspectionTaskDelReq{
		Id:        req.Id,
		UpdatedBy: currentUsername(l.ctx),
	})
	if err != nil {
		return "", err
	}

	return "删除巡检任务成功", nil
}
