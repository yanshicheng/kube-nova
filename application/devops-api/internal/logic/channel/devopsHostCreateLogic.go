// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package channel

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/channelservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsHostCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建主机资产
func NewDevopsHostCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsHostCreateLogic {
	return &DevopsHostCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsHostCreateLogic) DevopsHostCreate(req *types.CreateDevopsHostRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.HostCreate(l.ctx, &channelservice.CreateHostReq{
		Name:         req.Name,
		Ip:           req.Ip,
		Port:         req.Port,
		CredentialId: req.CredentialId,
		Labels:       req.Labels,
		Description:  req.Description,
		Status:       req.Status,
		CreatedBy:    currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
