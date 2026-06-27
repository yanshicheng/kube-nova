package channelservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CredentialDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCredentialDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CredentialDeleteLogic {
	return &CredentialDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CredentialDeleteLogic) CredentialDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.CredentialModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("凭证删除失败: %v", err)
		return nil, err
	}
	if err := ensureCredentialWriteAccess(l.ctx, l.svcCtx, data, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("凭证删除失败: %v", err)
		return nil, err
	}
	if normalizeCredentialScope(data.Scope, data.ProjectID, data.IsSystem) == "system" {
		l.Errorf("系统凭据不能删除")
		return nil, errorx.Msg("系统凭据不能删除")
	}
	if _, total, err := collectCredentialUsage(l.ctx, l.svcCtx, in.Id, 1); err != nil {
		l.Errorf("凭证删除失败: %v", err)
		return nil, err
	} else if total > 0 {
		l.Errorf("凭据正在被资源使用，不能删除")
		return nil, errorx.Msg("凭据正在被资源使用，不能删除")
	}
	if err := l.svcCtx.CredentialModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("凭证删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
