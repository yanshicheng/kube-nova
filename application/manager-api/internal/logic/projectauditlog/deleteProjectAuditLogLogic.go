package projectauditlog

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectAuditLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除指定的审计日志记录
func NewDeleteProjectAuditLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectAuditLogLogic {
	return &DeleteProjectAuditLogLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteProjectAuditLogLogic) DeleteProjectAuditLog(req *types.DeleteProjectAuditLogRequest) (resp string, err error) {
	// 调用 RPC 服务
	_, err = l.svcCtx.ManagerRpc.ProjectAuditLogDel(l.ctx, &pb.DelOnecProjectAuditLogReq{
		Id: req.Id,
	})

	if err != nil {
		l.Errorf("调用 RPC 服务失败: %v, id=%d", err, req.Id)
		return "", err
	}

	l.Infof("删除审计日志成功: id=%d", req.Id)
	return "删除审计日志成功", nil
}
