package projectservicelogic

import (
	"context"
	"strings"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeCreateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeCreateLogic {
	return &ConfigTypeCreateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeCreateLogic) ConfigTypeCreate(in *pb.CreateConfigTypeReq) (*pb.IdResp, error) {
	parentID := strings.TrimSpace(in.ParentId)
	name := strings.TrimSpace(in.Name)
	code := strings.TrimSpace(in.Code)
	if err := ensureConfigTypeParent(l.ctx, l.svcCtx, "", parentID); err != nil {
		l.Errorf("配置类型创建失败: %v", err)
		return nil, err
	}
	if err := validateConfigTypeBase(l.ctx, l.svcCtx, parentID, name, code, ""); err != nil {
		l.Errorf("配置类型创建失败: %v", err)
		return nil, err
	}
	data := &model.DevopsConfigType{
		ParentID:    parentID,
		Name:        name,
		Code:        code,
		StorageType: normalizeConfigStorageType(in.StorageType),
		Description: strings.TrimSpace(in.Description),
		SortOrder:   in.SortOrder,
		Status:      in.Status,
		CreatedBy:   in.CreatedBy,
		UpdatedBy:   in.CreatedBy,
	}
	if err := l.svcCtx.ConfigTypeModel.Insert(l.ctx, data); err != nil {
		l.Errorf("配置类型创建失败: %v", err)
		return nil, err
	}

	return &pb.IdResp{Id: data.ID.Hex()}, nil
}
