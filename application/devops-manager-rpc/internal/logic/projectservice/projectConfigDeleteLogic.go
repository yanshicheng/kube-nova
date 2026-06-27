package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectConfigDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectConfigDeleteLogic {
	return &ProjectConfigDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectConfigDeleteLogic) ProjectConfigDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目配置删除失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, data.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("项目配置删除失败: %v", err)
		return nil, err
	}
	if err := l.svcCtx.ProjectConfigModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("项目配置删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
