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

type DevopsChannelTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试渠道配置连通性
func NewDevopsChannelTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsChannelTestLogic {
	return &DevopsChannelTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsChannelTestLogic) DevopsChannelTest(req *types.TestDevopsChannelRequest) (resp *types.TestDevopsChannelResponse, err error) {
	result, err := l.svcCtx.ChannelRpc.ChannelTest(l.ctx, &channelservice.TestChannelReq{
		Id:              req.Id,
		GroupId:         req.GroupId,
		Name:            req.Name,
		Code:            req.Code,
		ChannelType:     req.ChannelType,
		Endpoint:        req.Endpoint,
		AuthType:        req.AuthType,
		Username:        req.Username,
		Password:        req.Password,
		Token:           req.Token,
		InsecureSkipTls: req.InsecureSkipTls,
		Config:          req.Config,
		CredentialId:    req.CredentialId,
	})
	if err != nil {
		return nil, err
	}

	return &types.TestDevopsChannelResponse{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
