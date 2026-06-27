package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMavenConfigDeleteLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMavenConfigDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMavenConfigDeleteLogic {
	return &ProjectMavenConfigDeleteLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMavenConfigDeleteLogic) ProjectMavenConfigDelete(in *pb.DeleteByIdReq) (*pb.EmptyResp, error) {
	data, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Maven 配置删除失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, data.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("Maven 配置删除失败: %v", err)
		return nil, err
	}
	if data.TypeCode != model.DefaultMavenSettingsTypeCode {
		l.Errorf("Maven 配置不存在")
		return nil, errorx.Msg("Maven 配置不存在")
	}
	if err := l.svcCtx.ProjectConfigModel.DeleteSoft(l.ctx, in.Id, in.UpdatedBy); err != nil {
		l.Errorf("Maven 配置删除失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
