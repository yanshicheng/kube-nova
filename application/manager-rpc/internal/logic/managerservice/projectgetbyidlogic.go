package managerservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectGetByIdLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectGetByIdLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectGetByIdLogic {
	return &ProjectGetByIdLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectGetByIdLogic) ProjectGetById(in *pb.GetOnecProjectByIdReq) (*pb.GetOnecProjectByIdResp, error) {
	if in.Id <= 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}
	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Logger.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	return &pb.GetOnecProjectByIdResp{
		Data: &pb.ProjectInfo{
			Id:          project.Id,
			Name:        project.Name,
			Uuid:        project.Uuid,
			Description: project.Description,
			CreatedBy:   project.CreatedBy,
			UpdatedBy:   project.UpdatedBy,
			CreatedAt:   project.CreatedAt.Unix(),
			UpdatedAt:   project.UpdatedAt.Unix(),
			IsSystem:    project.IsSystem,
		},
	}, nil
}
