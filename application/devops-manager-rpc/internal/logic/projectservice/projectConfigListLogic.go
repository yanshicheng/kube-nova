package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectConfigListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectConfigListLogic {
	return &ProjectConfigListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectConfigListLogic) ProjectConfigList(in *pb.ListProjectConfigReq) (*pb.ListProjectConfigResp, error) {
	if in.ProjectId != "" {
		if err := ensureProjectAccess(l.ctx, l.svcCtx, in.ProjectId, in.CurrentUserId, in.CurrentRoles); err != nil {
			l.Errorf("项目配置列表查询失败: %v", err)
			return nil, err
		}
	}
	projectIDs, restricted, err := memberProjectScope(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("项目配置列表查询失败: %v", err)
		return nil, err
	}
	data, total, err := l.svcCtx.ProjectConfigModel.List(l.ctx, model.DevopsProjectConfigListFilter{
		ProjectID:  in.ProjectId,
		ProjectIDs: projectIDs,
		Restricted: restricted,
		TypeID:     in.TypeId,
		TypeCode:   in.TypeCode,
		Name:       in.Name,
		Code:       in.Code,
		Status:     in.Status,
		Page:       in.Page,
		PageSize:   in.PageSize,
	})
	if err != nil {
		l.Errorf("项目配置列表查询失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsProjectConfig, 0, len(data))
	for _, item := range data {
		items = append(items, projectConfigToPb(item))
	}

	return &pb.ListProjectConfigResp{Data: items, Total: total}, nil
}
