package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeUpdateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeUpdateLogic {
	return &ConfigTypeUpdateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeUpdateLogic) ConfigTypeUpdate(in *pb.UpdateConfigTypeReq) (*pb.EmptyResp, error) {
	exist, err := l.svcCtx.ConfigTypeModel.FindOne(l.ctx, in.Id)
	if err != nil {
		l.Errorf("配置类型更新失败: %v", err)
		return nil, err
	}
	parentID := strings.TrimSpace(in.ParentId)
	name := strings.TrimSpace(in.Name)
	if err := ensureConfigTypeParent(l.ctx, l.svcCtx, in.Id, parentID); err != nil {
		l.Errorf("配置类型更新失败: %v", err)
		return nil, err
	}
	if err := validateConfigTypeBase(l.ctx, l.svcCtx, parentID, name, exist.Code, in.Id); err != nil {
		l.Errorf("配置类型更新失败: %v", err)
		return nil, err
	}
	oid, err := model.ObjectIDForUpdate(in.Id)
	if err != nil {
		l.Errorf("配置类型更新失败: %v", err)
		return nil, err
	}
	data := &model.DevopsConfigType{
		ID:          oid,
		ParentID:    parentID,
		Name:        name,
		Code:        exist.Code,
		StorageType: normalizeConfigStorageType(in.StorageType),
		Description: strings.TrimSpace(in.Description),
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		UpdatedBy:   in.UpdatedBy,
	}
	if err := l.svcCtx.ConfigTypeModel.Update(l.ctx, data); err != nil {
		l.Errorf("配置类型更新失败: %v", err)
		return nil, err
	}

	return &pb.EmptyResp{}, nil
}
