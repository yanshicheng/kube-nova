package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectDeleteLogic {
	return &ProjectDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectDeleteLogic) ProjectDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	if err := l.svcCtx.ProjectModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectChannelModel.DeleteSoftByProjectScope(l.ctx, in.Id, model.BuildUsageScope, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectMavenModel.DeleteSoftByProject(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectConfigModel.DeleteSoftByProject(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
