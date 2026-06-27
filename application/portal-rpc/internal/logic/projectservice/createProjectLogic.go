package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/portal-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/portal-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateProjectLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateProjectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateProjectLogic {
	return &CreateProjectLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateProjectLogic) CreateProject(in *pb.PortalCreateProjectReq) (*pb.PortalCreateProjectResp, error) {
	if in.Name == "" {
		return nil, errorx.Msg("项目名称不能为空")
	}

	project, err := l.svcCtx.OnecProjectModel.InsertWithUuid(l.ctx, in.Name, in.IsSystem, in.Description, in.CreatedBy)
	if err != nil {
		l.Errorf("创建项目失败: %v", err)
		return nil, errorx.Msg("创建项目失败")
	}

	l.Infof("创建项目成功，ID: %d, UUID: %s, Name: %s", project.Id, project.Uuid, project.Name)
	return &pb.PortalCreateProjectResp{
		Id:   project.Id,
		Uuid: project.Uuid,
	}, nil
}
