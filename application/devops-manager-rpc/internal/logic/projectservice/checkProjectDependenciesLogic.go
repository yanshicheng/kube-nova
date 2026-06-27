package projectservicelogic

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type CheckProjectDependenciesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCheckProjectDependenciesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CheckProjectDependenciesLogic {
	return &CheckProjectDependenciesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CheckProjectDependenciesLogic) CheckProjectDependencies(in *pb.DevopsCheckProjectDependenciesReq) (*pb.DevopsCheckProjectDependenciesResp, error) {
	var deps []*pb.DevopsProjectDependency

	// 通过 portalProjectUuid 查找 devops 项目
	project, err := l.svcCtx.ProjectModel.FindOneByPortalUuid(l.ctx, in.PortalProjectUuid)
	if err != nil {
		// 项目不存在，无依赖
		l.Infof("DevOps 项目不存在，portalProjectUuid: %s", in.PortalProjectUuid)
		return &pb.DevopsCheckProjectDependenciesResp{}, nil
	}
	projectId := project.ID.Hex()

	// 检查 system
	_, sysTotal, err := l.svcCtx.SystemModel.List(l.ctx, model.DevopsSystemListFilter{
		ProjectID: projectId, Page: 1, PageSize: 1,
	})
	if err == nil && sysTotal > 0 {
		deps = append(deps, &pb.DevopsProjectDependency{
			ResourceType: "devops_system",
			Count:        int64(sysTotal),
			Message:      fmt.Sprintf("业务系统 (%d个)", sysTotal),
		})
	}

	// 检查 credential (scope=project)
	_, credTotal, err := l.svcCtx.CredentialModel.List(l.ctx, model.DevopsCredentialListFilter{
		Scope: "project", ProjectID: projectId, Page: 1, PageSize: 1,
	})
	if err == nil && credTotal > 0 {
		deps = append(deps, &pb.DevopsProjectDependency{
			ResourceType: "devops_credential",
			Count:        int64(credTotal),
			Message:      fmt.Sprintf("项目凭证 (%d个)", credTotal),
		})
	}

	// 检查 channel_binding
	_, bindTotal, err := l.svcCtx.ProjectChannelModel.List(l.ctx, model.DevopsProjectChannelBindingListFilter{
		ProjectID: projectId, Page: 1, PageSize: 1,
	})
	if err == nil && bindTotal > 0 {
		deps = append(deps, &pb.DevopsProjectDependency{
			ResourceType: "devops_channel_binding",
			Count:        int64(bindTotal),
			Message:      fmt.Sprintf("渠道绑定 (%d个)", bindTotal),
		})
	}

	// 检查 member
	_, memTotal, err := l.svcCtx.ProjectMemberModel.List(l.ctx, model.DevopsProjectMemberListFilter{
		ProjectID: projectId, Page: 1, PageSize: 1,
	})
	if err == nil && memTotal > 0 {
		deps = append(deps, &pb.DevopsProjectDependency{
			ResourceType: "devops_project_member",
			Count:        int64(memTotal),
			Message:      fmt.Sprintf("项目成员 (%d个)", memTotal),
		})
	}

	// 检查 pipeline_template
	_, tmplTotal, err := l.svcCtx.PipelineTemplateModel.List(l.ctx, model.DevopsPipelineTemplateListFilter{
		ProjectID: projectId, Page: 1, PageSize: 1,
	})
	if err == nil && tmplTotal > 0 {
		deps = append(deps, &pb.DevopsProjectDependency{
			ResourceType: "devops_pipeline_template",
			Count:        int64(tmplTotal),
			Message:      fmt.Sprintf("流水线模板 (%d个)", tmplTotal),
		})
	}

	// 检查 host (通过 credential 间接关联，这里检查是否有项目级主机)
	_, hostTotal, err := l.svcCtx.HostModel.List(l.ctx, model.DevopsHostListFilter{
		Page: 1, PageSize: 1,
	})
	if err == nil && hostTotal > 0 {
		// host 没有直接 projectId 字段，暂不检查
		// 实际环境中 host 通过 credential 间接关联项目
	}

	l.Infof("DevOps 项目依赖检查完成，portalProjectUuid: %s, 依赖数: %d", in.PortalProjectUuid, len(deps))
	return &pb.DevopsCheckProjectDependenciesResp{Dependencies: deps}, nil
}
