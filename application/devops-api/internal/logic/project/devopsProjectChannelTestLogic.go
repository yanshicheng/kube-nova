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

type DevopsProjectChannelTestLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 测试项目渠道绑定
func NewDevopsProjectChannelTestLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsProjectChannelTestLogic {
	return &DevopsProjectChannelTestLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsProjectChannelTestLogic) DevopsProjectChannelTest(req *types.DefaultStringIdRequest) (resp *types.TestDevopsProjectChannelResponse, err error) {
	result, err := l.svcCtx.ProjectRpc.ProjectChannelTest(l.ctx, &projectservice.GetByIdReq{
		Id:            req.Id,
		CurrentUserId: currentUserID(l.ctx),
		CurrentRoles:  currentRoles(l.ctx),
	})
	if err != nil {
		return nil, err
	}

	return &types.TestDevopsProjectChannelResponse{
		Success:      result.Success,
		HealthStatus: result.HealthStatus,
		Message:      result.Message,
		CheckedAt:    result.CheckedAt,
		Metadata:     result.Metadata,
	}, nil
}
