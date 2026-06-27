package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectDeleteLogic {
	return &ProjectDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ProjectDelete 删除 DevOps 项目（仅删除 DevOps 扩展数据，portal 项目由 portal 统一管理）
func (l *ProjectDeleteLogic) ProjectDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	// 获取项目信息
	project, err := l.svcCtx.ProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目不存在: %v", err)
		return nil, errorx.Msg("项目不存在")
	}

	// 只删除 DevOps 扩展数据，不删除 portal 项目
	if err := l.svcCtx.ProjectModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectChannelModel.DeleteSoftByProjectScope(l.ctx, in.Id, model.BuildUsageScope, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectMavenModel.DeleteSoftByProject(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectConfigModel.DeleteSoftByProject(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}

	l.Infof("DevOps 项目扩展数据已删除，portalProjectUuid: %s", project.PortalProjectUuid)
	return &pb.EmptyResp{}, nil
}
