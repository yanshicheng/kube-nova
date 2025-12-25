package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteAlertInstanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除告警实例
func NewDeleteAlertInstanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteAlertInstanceLogic {
	return &DeleteAlertInstanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteAlertInstanceLogic) DeleteAlertInstance(req *types.DefaultIdRequest) (resp string, err error) {
	// 调用RPC服务删除告警实例
	_, err = l.svcCtx.ManagerRpc.AlertInstancesDel(l.ctx, &pb.DelAlertInstancesReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("删除告警实例失败: %v", err)
		return "", fmt.Errorf("删除告警实例失败: %v", err)
	}

	return "告警实例删除成功", nil
}
