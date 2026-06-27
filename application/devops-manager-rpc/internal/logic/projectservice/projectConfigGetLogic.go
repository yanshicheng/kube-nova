package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectConfigGetLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectConfigGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectConfigGetLogic {
	return &ProjectConfigGetLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectConfigGetLogic) ProjectConfigGet(in *pb.GetByIdReq) (*pb.GetProjectConfigResp, error) {
	data, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目配置查询失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, data.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("项目配置查询失败: %v", err)
		return nil, err
	}

	return &pb.GetProjectConfigResp{Data: projectConfigToPb(data)}, nil
}
