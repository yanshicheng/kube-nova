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

type DevopsTektonSecretGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取项目 Tekton Secret
func NewDevopsTektonSecretGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsTektonSecretGetLogic {
	return &DevopsTektonSecretGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsTektonSecretGetLogic) DevopsTektonSecretGet(req *types.GetDevopsTektonSecretRequest) (resp *types.TektonSecret, err error) {
	result, err := l.svcCtx.ProjectRpc.TektonSecretGet(l.ctx, &projectservice.GetTektonSecretReq{
		ProjectId:     req.ProjectId,
		BindingId:     req.BindingId,
		Name:          req.Name,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}
	data := tektonSecretToType(result.Data)

	return &data, nil
}
