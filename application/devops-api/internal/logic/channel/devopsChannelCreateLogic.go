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

type DevopsChannelCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 创建渠道
func NewDevopsChannelCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelCreateLogic {
	return &DevopsChannelCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelCreateLogic) DevopsChannelCreate(req *types.CreateDevopsChannelRequest) (resp *types.IdResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelCreate(l.ctx, &channelservice.CreateChannelReq{
		GroupId:            req.GroupId,
		Name:               req.Name,
		Code:               req.Code,
		ChannelType:        req.ChannelType,
		Endpoint:           req.Endpoint,
		Description:        req.Description,
		GlobalCredentialId: req.GlobalCredentialId,
		CredentialId:       req.CredentialId,
		Config:             req.Config,
		Labels:             req.Labels,
		AuthType:           req.AuthType,
		Username:           req.Username,
		Password:           req.Password,
		Token:              req.Token,
		InsecureSkipTls:    req.InsecureSkipTls,
		Icon:               req.Icon,
		IconColor:          req.IconColor,
		Status:             req.Status,
		CreatedBy:          currentUsername(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.IdResponse{Id: result.Id}, nil
}
