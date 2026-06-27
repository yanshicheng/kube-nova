package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMemberListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMemberListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMemberListLogic {
	return &ProjectMemberListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMemberListLogic) ProjectMemberList(in *pb.ListProjectMemberReq) (*pb.ListProjectMemberResp, error) {
	data, total, err := l.svcCtx.ProjectMemberModel.List(l.ctx, model.DevopsProjectMemberListFilter{
		ProjectID: in.ProjectId,
		UserID:    in.UserId,
		Role:      in.Role,
		Status:    in.Status,
		Page:      in.Page,
		PageSize:  in.PageSize,
	})
	if err != nil {
		l.Errorf("项目成员查询列表失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsProjectMember, 0, len(data))
	for _, item := range data {
		items = append(items, projectMemberToPb(item))
	}

	return &pb.ListProjectMemberResp{Data: items, Total: total}, nil
}
