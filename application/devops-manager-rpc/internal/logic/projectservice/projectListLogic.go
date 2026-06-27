package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectListLogic {
	return &ProjectListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectListLogic) ProjectList(in *pb.ListProjectReq) (*pb.ListProjectResp, error) {
	projectIDs, restricted, err := memberProjectScope(l.ctx, l.svcCtx, in.CurrentUserId, in.CurrentRoles)
	if err != nil {
		l.Errorf("项目查询列表失败: %v", err)
		return nil, err
	}
	data, total, err := l.svcCtx.ProjectModel.List(l.ctx, model.DevopsProjectListFilter{
		Name:               in.Name,
		Code:               in.Code,
		PipelineEngineType: in.PipelineEngineType,
		Status:             in.Status,
		ProjectIDs:         projectIDs,
		Restricted:         restricted,
		Page:               in.Page,
		PageSize:           in.PageSize,
	})
	if err != nil {
		l.Errorf("项目查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsProject, 0, len(data))
	for _, item := range data {
		items = append(items, projectToPb(item))
	}

	return &pb.ListProjectResp{Data: items, Total: total}, nil
}
