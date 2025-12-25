package projectauditlog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectAuditLogBeforeDaysLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除指定天数之前的审计日志数据，用于定期清理历史数据
func NewDeleteProjectAuditLogBeforeDaysLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectAuditLogBeforeDaysLogic {
	return &DeleteProjectAuditLogBeforeDaysLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectAuditLogBeforeDaysLogic) DeleteProjectAuditLogBeforeDays(req *types.DeleteProjectAuditLogBeforeDaysRequest) (resp string, err error) {
	// 调用 RPC 服务
	_, err = l.svcCtx.ManagerRpc.ProjectAuditLogDelBeforeDays(l.ctx, &pb.DelOnecProjectAuditLogBeforeDaysReq{
		Days: req.Days,
	})

	if err != nil {
		l.Errorf("调用 RPC 服务失败: %v, days=%d", err, req.Days)
		return "", err
	}

	l.Infof("删除指定天数之前的审计日志成功: days=%d", req.Days)
	return "删除指定天数之前的审计日志成功", nil
}
