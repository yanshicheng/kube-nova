package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectConfigCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectConfigCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectConfigCreateLogic {
	return &ProjectConfigCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectConfigCreateLogic) ProjectConfigCreate(in *pb.CreateProjectConfigReq) (*pb.IdResp, error) {
	projectID := strings.TrimSpace(in.ProjectId)
	if err := ensureProjectAccess(l.ctx, l.svcCtx, projectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("项目配置创建失败: %v", err)
		return nil, err
	}
	configType, err := normalizeConfigTypeRef(l.ctx, l.svcCtx, in.TypeId, in.TypeCode)
	if err != nil {
		l.Errorf("项目配置创建失败: %v", err)
		return nil, err
	}
	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)
	content := strings.TrimSpace(in.Content)
	if err := validateProjectConfigBase(l.ctx, l.svcCtx, projectID, configType.Code, name, code, content, "", "项目配置"); err != nil {
		l.Errorf("项目配置创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsProjectConfig{
		ProjectID:   projectID,
		TypeID:      configType.ID.Hex(),
		TypeCode:    configType.Code,
		TypeName:    configType.Name,
		Name:        name,
		Code:        code,
		Content:     content,
		Description: strings.TrimSpace(in.Description),
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
	}
	if err := l.svcCtx.ProjectConfigModel.Insert(l.ctx, data); err != nil {
		l.Errorf("项目配置创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
