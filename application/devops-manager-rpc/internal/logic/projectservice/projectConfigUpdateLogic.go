package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProjectConfigUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewProjectConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProjectConfigUpdateLogic {
	return &ProjectConfigUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ProjectConfigUpdateLogic) ProjectConfigUpdate(in *pb.UpdateProjectConfigReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.ProjectConfigModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("项目配置更新失败: %v", err)
		return nil, err
	}
	if err := ensureProjectAccess(l.ctx, l.svcCtx, exist.ProjectID, in.CurrentUserId, in.CurrentRoles); err != nil {
		l.Errorf("项目配置更新失败: %v", err)
		return nil, err
	}
	typeID := exist.TypeID
	typeCode := exist.TypeCode
	typeName := exist.TypeName
	if strings.TrimSpace(in.TypeId) != "" || strings.TrimSpace(in.TypeCode) != "" {
		configType, err := normalizeConfigTypeRef(l.ctx, l.svcCtx, in.TypeId, in.TypeCode)
		if err != nil {
			l.Errorf("项目配置更新失败: %v", err)
			return nil, err
		}
		typeID = configType.ID.Hex()
		typeCode = configType.Code
		typeName = configType.Name
	}
	name := strings.TrimSpace(in.Name)
	content := strings.TrimSpace(in.Content)
	if err := validateProjectConfigBase(l.ctx, l.svcCtx, exist.ProjectID, typeCode, name, exist.Code, content, in.Id, "项目配置"); err != nil {
		l.Errorf("项目配置更新失败: %v", err)
		return nil, err
	}
	oid, err := model.ObjectIDForUpdate(in.Id)
	if err != nil {
		l.Errorf("项目配置更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsProjectConfig{
		ID:          oid,
		TypeID:      typeID,
		TypeCode:    typeCode,
		TypeName:    typeName,
		Name:        name,
		Code:        exist.Code,
		Content:     content,
		Description: strings.TrimSpace(in.Description),
		Status:      in.Status,
		UpdatedBy:   in.UpdatedBy,
	}
	if err := l.svcCtx.ProjectConfigModel.Update(l.ctx, data); err != nil {
		l.Errorf("项目配置更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
