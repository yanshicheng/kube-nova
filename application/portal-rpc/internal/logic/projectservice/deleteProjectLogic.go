package projectservicelogic

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	managerpb "github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	devopspb "github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteProjectLogic {
	return &DeleteProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteProjectLogic) DeleteProject(in *pb.PortalDeleteProjectReq) (*pb.PortalDeleteProjectResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	// 1. 检查项目是否存在
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	// 2. 检查 manager 依赖
	mgrResp, mgrErr := l.svcCtx.ManagerRpc.CheckProjectDependencies(l.ctx, &managerpb.ManagerCheckProjectDependenciesReq{
		ProjectId: in.Id,
	})

	// 3. 检查 devops 依赖
	devResp, devErr := l.svcCtx.DevopsManagerRpc.CheckProjectDependencies(l.ctx, &devopspb.DevopsCheckProjectDependenciesReq{
		PortalProjectUuid: project.Uuid,
	})

	// 4. 汇总依赖
	var allDeps []string
	if mgrErr == nil && mgrResp != nil {
		for _, d := range mgrResp.Dependencies {
			allDeps = append(allDeps, d.Message)
		}
	}
	if devErr == nil && devResp != nil {
		for _, d := range devResp.Dependencies {
			allDeps = append(allDeps, d.Message)
		}
	}

	if len(allDeps) > 0 {
		l.Errorf("项目存在依赖，无法删除，ID: %d, 依赖: %v", in.Id, allDeps)
		return nil, errorx.Msg(fmt.Sprintf("项目存在关联数据，请先清理: %s", strings.Join(allDeps, "、")))
	}

	// 5. 执行软删除
	err = l.svcCtx.OnecProjectModel.DeleteSoft(l.ctx, in.Id)
	if err != nil {
		l.Errorf("删除项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("删除项目失败")
	}

	// 6. 通知 devops 同步删除
	updatedBy := in.UpdatedBy
	if updatedBy == "" {
		updatedBy = "system"
	}
	_, syncErr := l.svcCtx.DevopsManagerRpc.SyncProjectDeleted(l.ctx, &devopspb.DevopsSyncProjectDeletedReq{
		PortalProjectUuid: project.Uuid,
		UpdatedBy:         updatedBy,
	})
	if syncErr != nil {
		l.Errorf("同步 DevOps 项目删除失败，portalProjectUuid: %s, 错误: %v", project.Uuid, syncErr)
	}

	l.Infof("删除项目成功，ID: %d, UUID: %s, Name: %s", project.Id, project.Uuid, project.Name)
	return &pb.PortalDeleteProjectResp{}, nil
}
