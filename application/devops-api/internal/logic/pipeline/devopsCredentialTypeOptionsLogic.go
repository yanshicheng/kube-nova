// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package pipeline

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/devops-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/devops-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/devops-manager-rpc/client/pipelineconfigservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type DevopsCredentialTypeOptionsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 凭证类型选项
func NewDevopsCredentialTypeOptionsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DevopsCredentialTypeOptionsLogic {
	return &DevopsCredentialTypeOptionsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DevopsCredentialTypeOptionsLogic) DevopsCredentialTypeOptions() (resp *types.CredentialTypeOptionsResponse, err error) {
	result, err := l.svcCtx.PipelineRpc.CredentialTypeOptions(l.ctx, &pipelineconfigservice.EmptyResp{})
	if err != nil {
		return nil, err
	}
	items := make([]types.CredentialTypeOption, 0, len(result.Data))
	for _, item := range result.Data {
		if item == nil {
			continue
		}
		items = append(items, types.CredentialTypeOption{
			CredentialType:      item.CredentialType,
			Name:                item.Name,
			MappingFields:       item.MappingFields,
			DefaultMappingField: item.DefaultMappingField,
		})
	}

	return &types.CredentialTypeOptionsResponse{Items: items}, nil
}
