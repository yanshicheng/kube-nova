// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package project

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindProjectPlatformLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 解绑项目平台
func NewUnbindProjectPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindProjectPlatformLogic {
	return &UnbindProjectPlatformLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnbindProjectPlatformLogic) UnbindProjectPlatform(req *types.UnbindProjectPlatformReq) error {
	if !isSuperAdmin(currentRoles(l.ctx)) {
		return errorx.Msg("无项目平台授权权限")
	}
	_, err := l.svcCtx.ProjectRpc.UnbindProjectPlatform(l.ctx, &pb.UnbindProjectPlatformReq{
		ProjectId:  req.Id,
		PlatformId: req.PlatformId,
		UpdatedBy:  currentUsername(l.ctx),
	})
	if err != nil {
		l.Errorf("解绑项目平台失败: %v", err)
		return err
	}

	return nil
}
