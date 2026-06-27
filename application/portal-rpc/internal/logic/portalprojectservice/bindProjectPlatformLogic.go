package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type BindProjectPlatformLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindProjectPlatformLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindProjectPlatformLogic {
	return &BindProjectPlatformLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *BindProjectPlatformLogic) BindProjectPlatform(in *pb.BindProjectPlatformReq) (*pb.BindProjectPlatformResp, error) {
	if in.ProjectId == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	if in.PlatformId == 0 {
		return nil, errorx.Msg("平台ID不能为空")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.ProjectId)
	if err != nil {
		l.Errorf("绑定项目平台失败，项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}
	if project.IsSystem == 1 {
		return nil, errorx.Msg("平台项目不允许修改平台授权")
	}
	platform, err := l.svcCtx.SysPlatformModel.FindOne(l.ctx, in.PlatformId)
	if err != nil {
		l.Errorf("绑定项目平台失败，平台不存在: %v", err)
		return nil, errorx.Msg("平台不存在")
	} else if platform.IsEnable != 1 || platform.IsDeleted != 0 {
		return nil, errorx.Msg("平台不可用")
	}
	if err := l.svcCtx.ProjectPlatformBindingModel.BindProjectPlatform(l.ctx, in.ProjectId, in.PlatformId, in.CreatedBy); err != nil {
		l.Errorf("绑定项目平台失败: %v", err)
		return nil, errorx.Msg("绑定项目平台失败")
	}
	if err := l.svcCtx.ProjectMemberPlatformRole.GrantPlatformToProjectMembers(l.ctx, in.ProjectId, in.PlatformId, in.CreatedBy); err != nil {
		l.Errorf("同步项目成员平台授权失败: %v", err)
		return nil, errorx.Msg("绑定项目平台失败")
	}
	if isDevopsPlatform(platform) {
		if syncErr := syncDevopsProjectInfo(l.ctx, l.svcCtx, project, in.CreatedBy); syncErr != nil {
			l.Errorf("同步 DevOps 项目信息失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
			return nil, errorx.Msg("同步 DevOps 项目失败")
		}
		if syncErr := syncDevopsProjectMembers(l.ctx, l.svcCtx, project, in.CreatedBy); syncErr != nil {
			l.Errorf("同步 DevOps 项目成员失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
			return nil, errorx.Msg("同步 DevOps 项目成员失败")
		}
	}

	return &pb.BindProjectPlatformResp{}, nil
}
