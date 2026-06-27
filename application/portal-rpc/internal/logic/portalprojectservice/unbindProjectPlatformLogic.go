package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindProjectPlatformLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnbindProjectPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindProjectPlatformLogic {
	return &UnbindProjectPlatformLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UnbindProjectPlatformLogic) UnbindProjectPlatform(in *pb.UnbindProjectPlatformReq) (*pb.UnbindProjectPlatformResp, error) {
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	if in.PlatformId == 0 {
		return nil, errorx.Msg("平台ID不能为空")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("解绑项目平台失败，项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}
	if project.IsSystem == 1 {
		return nil, errorx.Msg("平台项目不允许修改平台授权")
	}
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.PlatformId)
	if err != nil {
		l.Errorf("解绑项目平台失败，平台不存在: %v", err)
		return nil, errorx.Msg("平台不存在")
	}
	if isPortalPlatform(platform) {
		return nil, errorx.Msg("门户平台为项目默认平台，不允许解绑")
	}
	if err := l.svcCtx.ProjectPlatformBindingModel.UnbindProjectPlatform(l.ctx, in.ProjectId, in.PlatformId, in.UpdatedBy); err != nil {
		l.Errorf("解绑项目平台失败: %v", err)
		return nil, errorx.Msg("解绑项目平台失败")
	}
	if err := l.svcCtx.ProjectMemberPlatformRole.RevokeProjectPlatform(l.ctx, in.ProjectId, in.PlatformId, in.UpdatedBy); err != nil {
		l.Errorf("回收项目成员平台授权失败: %v", err)
		return nil, errorx.Msg("解绑项目平台失败")
	}
	if isDevopsPlatform(platform) {
		if syncErr := syncDevopsProjectDeleted(l.ctx, l.svcCtx, project, in.UpdatedBy); syncErr != nil {
			l.Errorf("同步 DevOps 项目删除失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
			return nil, errorx.Msg("同步 DevOps 项目失败")
		}
	}

	return &pb.UnbindProjectPlatformResp{}, nil
}
