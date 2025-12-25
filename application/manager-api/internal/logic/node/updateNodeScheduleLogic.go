package node

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateNodeScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateNodeScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateNodeScheduleLogic {
	return &UpdateNodeScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateNodeScheduleLogic) UpdateNodeSchedule(req *types.NodeDisableScheduleRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}
	_, err = l.svcCtx.ManagerRpc.NodeDisable(l.ctx, &pb.ClusterNodeDisableReq{
		Id:        req.Id,
		Status:    req.Status,
		UpdatedBy: username,
	})
	if err != nil {
		l.Errorf("更新节点调度状态失败: %v", err)
		return "", errorx.Msg("更新节点调度状态失败")
	}

	return "操作成功", nil
}
