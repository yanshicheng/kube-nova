package registry

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/console-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/console-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/console-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGCScheduleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 更新GC调度配置
func NewUpdateGCScheduleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGCScheduleLogic {
	return &UpdateGCScheduleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateGCScheduleLogic) UpdateGCSchedule(req *types.UpdateGCScheduleRequest) (resp string, err error) {
	_, err = l.svcCtx.RepositoryRpc.UpdateGCSchedule(l.ctx, &pb.UpdateGCScheduleReq{
		RegistryUuid:   req.RegistryUuid,
		Schedule:       req.Schedule,
		DeleteUntagged: req.DeleteUntagged,
	})
	if err != nil {
		l.Errorf("RPC调用失败: %v", err)
		return "", err
	}

	l.Infof("GC调度配置更新成功")
	return "GC调度配置更新成功", nil
}
