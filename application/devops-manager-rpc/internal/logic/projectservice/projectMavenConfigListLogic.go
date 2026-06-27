package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMavenConfigListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMavenConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMavenConfigListLogic {
	return &ProjectMavenConfigListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMavenConfigListLogic) ProjectMavenConfigList(in *pb.ListProjectMavenConfigReq) (*pb.ListProjectMavenConfigResp, error) {
	if in.ProjectId != "" {
		if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("Maven 配置列表查询失败: %v", err)
			return nil, err
		}
	}
	projectIDs, restricted, err := memberProjectScope(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("Maven 配置列表查询失败: %v", err)
		return nil, err
	}
	data, total, err := l.svcCtx.ProjectConfigModel.List(l.ctx, model.DevopsProjectConfigListFilter{
		ProjectID:  in.ProjectId,
		ProjectIDs: projectIDs,
		Restricted: restricted,
		TypeCode:   model.DefaultMavenSettingsTypeCode,
		Name:       in.Name,
		Code:       in.Code,
		Status:     in.Status,
		Page:       in.Page,
		PageSize:   in.PageSize,
	})
	if err != nil {
		l.Errorf("Maven 配置列表查询失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsProjectMavenConfig, 0, len(data))
	for _, item := range data {
		items = append(items, projectMavenConfigToPb(item))
	}

	return &pb.ListProjectMavenConfigResp{Data: items, Total: total}, nil
}
