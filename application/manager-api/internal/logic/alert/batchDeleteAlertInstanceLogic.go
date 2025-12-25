package alert

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDeleteAlertInstanceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除告警实例
func NewBatchDeleteAlertInstanceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteAlertInstanceLogic {
	return &BatchDeleteAlertInstanceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteAlertInstanceLogic) BatchDeleteAlertInstance(req *types.BatchDeleteAlertInstanceRequest) (resp string, err error) {
	// 调用RPC服务批量删除告警实例
	_, err = l.svcCtx.ManagerRpc.AlertInstancesBatchDel(l.ctx, &pb.BatchDelAlertInstancesReq{
		Ids: req.Ids,
	})

	if err != nil {
		l.Errorf("批量删除告警实例失败: %v", err)
		return "", fmt.Errorf("批量删除告警实例失败: %v", err)
	}

	return fmt.Sprintf("成功批量删除 %d 个告警实例", len(req.Ids)), nil
}
