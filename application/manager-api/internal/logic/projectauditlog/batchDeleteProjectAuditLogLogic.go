package projectauditlog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type BatchDeleteProjectAuditLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 批量删除审计日志记录，最多100条
func NewBatchDeleteProjectAuditLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BatchDeleteProjectAuditLogLogic {
	return &BatchDeleteProjectAuditLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *BatchDeleteProjectAuditLogLogic) BatchDeleteProjectAuditLog(req *types.BatchDeleteProjectAuditLogRequest) (resp string, err error) {
	// 调用 RPC 服务
	_, err = l.svcCtx.ManagerRpc.ProjectAuditLogBatchDel(l.ctx, &pb.BatchDelOnecProjectAuditLogReq{
		Ids: req.Ids,
	})

	if err != nil {
		l.Errorf("调用 RPC 服务失败: %v, ids count=%d", err, len(req.Ids))
		return "", err
	}

	l.Infof("批量删除审计日志成功: count=%d", len(req.Ids))
	return "批量删除审计日志成功", nil
}
