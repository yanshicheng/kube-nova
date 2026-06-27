package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetProjectLogic {
	return &GetProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetProjectLogic) GetProject(in *pb.PortalGetProjectReq) (*pb.PortalGetProjectResp, error) {
	if in.Id == 0 {
		return nil, errorx.Msg("项目ID不能为空")
	}

	project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("查询项目失败，ID: %d, 错误: %v", in.Id, err)
		return nil, errorx.Msg("项目不存在")
	}

	return &pb.PortalGetProjectResp{
		Data: &pb.PortalProject{
			Id:          project.Id,
			Name:        project.Name,
			Uuid:        project.Uuid,
			IsSystem:    project.IsSystem,
			Description: project.Description,
			CreatedBy:   project.CreatedBy,
			UpdatedBy:   project.UpdatedBy,
			CreatedAt:   project.CreatedAt.Unix(),
			UpdatedAt:   project.UpdatedAt.Unix(),
		},
	}, nil
}
