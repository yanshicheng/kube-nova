package portalprojectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	devopspb "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProjectLogic {
	return &UpdateProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateProjectLogic) UpdateProject(in *pb.PortalUpdateProjectReq) (*pb.PortalUpdateProjectResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	if in.Name != "" {
		project.Name = in.Name
	}
	if in.Description != "" {
		project.Description = in.Description
	}
	if in.UpdatedBy != "" {
		project.UpdatedBy = in.UpdatedBy
	}

	err = l.svcCtx.OnecProjectModel.Update(l.ctx, project)
	if err != nil {
		l.Errorf("更新项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("更新项目失败")
	}

	// 同步 DevOps 项目信息
	_, syncErr := l.svcCtx.DevopsManagerRpc.SyncProjectInfo(l.ctx, &devopspb.DevopsSyncProjectInfoReq{
		PortalProjectUuid: project.Uuid,
		Name:              project.Name,
		Description:       project.Description,
	})
	if syncErr != nil {
		l.Errorf("同步 DevOps 项目信息失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
	}

	l.Infof("更新项目成功，ID: %d, Name: %s", project.Id, project.Name)
	return &pb.PortalUpdateProjectResp{}, nil
}
