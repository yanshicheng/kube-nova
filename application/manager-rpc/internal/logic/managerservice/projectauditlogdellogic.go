package managerservicelogic

import (
	"context"
	"errors"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectAuditLogDelLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectAuditLogDelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectAuditLogDelLogic {
	return &ProjectAuditLogDelLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectAuditLogDelLogic) ProjectAuditLogDel(in *pb.DelOnecProjectAuditLogReq) (*pb.DelOnecProjectAuditLogResp, error) {
	// 参数校验
	if in.Id == 0 {
		l.Errorf("参数校验失败: id 不能为空")
		return nil, errorx.Msg("审计日志ID不能为空")
	}

	// 查询审计日志是否存在
	_, err := l.svcCtx.OnecProjectAuditLog.FindOne(l.ctx, in.Id)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			l.Errorf("审计日志不存在: id=%d", in.Id)
			return nil, errorx.Msg("审计日志不存在")
		}
		l.Errorf("查询审计日志失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("查询审计日志失败")
	}

	// 执行软删除
	err = l.svcCtx.OnecProjectAuditLog.Delete(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除审计日志失败: %v, id=%d", err, in.Id)
		return nil, errorx.Msg("删除审计日志失败")
	}

	return &pb.DelOnecProjectAuditLogResp{}, nil
}
