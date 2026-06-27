package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectMavenConfigUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectMavenConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectMavenConfigUpdateLogic {
	return &ProjectMavenConfigUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectMavenConfigUpdateLogic) ProjectMavenConfigUpdate(in *pb.UpdateProjectMavenConfigReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("Maven 配置更新失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, exist.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("Maven 配置更新失败: %v", err)
		return nil, err
	}
	if exist.TypeCode != model.DefaultMavenSettingsTypeCode {
		l.Errorf("Maven 配置不存在")
		return nil, errorx.Msg("Maven 配置不存在")
	}
	oid, err := model.ObjectIDForUpdate(in.Id)
	if err != nil {
		l.Errorf("Maven 配置更新失败: %v", err)
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	content := strings.TrimSpace(in.Content)
	if err := validateMavenConfigBase(l.ctx, l.svcCtx, exist.ProjectID, name, exist.Code, content, in.Id); err != nil {
		l.Errorf("Maven 配置更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsProjectConfig{
		ID:          oid,
		TypeID:      exist.TypeID,
		TypeCode:    exist.TypeCode,
		TypeName:    exist.TypeName,
		Name:        name,
		Code:        exist.Code,
		Content:     content,
		Description: strings.TrimSpace(in.Description),
		Status:      in.Status,
		UpdatedBy:   in.UpdatedBy,
	}
	if err := l.svcCtx.ProjectConfigModel.Update(l.ctx, data); err != nil {
		l.Errorf("Maven 配置更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
