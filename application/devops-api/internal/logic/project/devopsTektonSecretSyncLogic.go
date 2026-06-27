// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/projectservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsTektonSecretSyncLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 同步项目 Tekton Secret 到其他渠道
func NewDevopsTektonSecretSyncLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretSyncLogic {
	return &DevopsTektonSecretSyncLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretSyncLogic) DevopsTektonSecretSync(req *types.SyncDevopsTektonSecretRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonSecretSync(l.ctx, &projectservice.SyncTektonSecretReq{
		ProjectId:        req.ProjectId,
		SourceBindingId:  req.SourceBindingId,
		Name:             req.Name,
		TargetBindingIds: req.TargetBindingIds,
		Operator:         currentUsername(l.ctx),
		CurrentUserId:    currentUserID(l.ctx),
		CurrentRoles:     currentRoles(l.ctx),
	})

	return err
}
