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

type DevopsTektonSecretDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 删除项目 Tekton Secret
func NewDevopsTektonSecretDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretDeleteLogic {
	return &DevopsTektonSecretDeleteLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretDeleteLogic) DevopsTektonSecretDelete(req *types.DeleteDevopsTektonSecretRequest) error {
	_, err := l.svcCtx.ProjectRpc.TektonSecretDelete(l.ctx, &projectservice.DeleteTektonSecretReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		Name:          req.Name,
		Operator:      currentUsername(l.ctx),
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})

	return err
}
