package projectservicelogic

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/pb"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigTypeListLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConfigTypeListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigTypeListLogic {
	return &ConfigTypeListLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ConfigTypeListLogic) ConfigTypeList(in *pb.ListConfigTypeReq) (*pb.ListConfigTypeResp, error) {
	data, total, err := l.svcCtx.ConfigTypeModel.List(l.ctx, model.DevopsConfigTypeListFilter{
		ParentID: in.ParentId,
		Name:     in.Name,
		Code:     in.Code,
		Status:   in.Status,
		Page:     in.Page,
		PageSize: in.PageSize,
	})
	if err != nil {
		l.Errorf("配置类型列表查询失败: %v", err)
		return nil, err
	}
	items := make([]*pb.DevopsConfigType, 0, len(data))
	for _, item := range data {
		items = append(items, configTypeToPb(item))
	}

	return &pb.ListConfigTypeResp{Data: items, Total: total}, nil
}
