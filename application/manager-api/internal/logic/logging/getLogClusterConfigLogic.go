// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logging

import (
	"context"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	logservice "github.com/yanshicheng/kube-nova/application/manager-rpc/client/logservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetLogClusterConfigLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 查询集群日志后端配置摘要
func NewGetLogClusterConfigLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetLogClusterConfigLogic {
	return &GetLogClusterConfigLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetLogClusterConfigLogic) GetLogClusterConfig(req *types.LogClusterConfigRequest) (resp *types.LogClusterConfigResponse, err error) {
	rpcResp, err := l.svcCtx.LogRpc.GetLogClusterConfig(l.ctx, &logservice.GetLogClusterConfigReq{
		ClusterUuid: req.ClusterUuid,
	})
	if err != nil {
		return nil, err
	}

	items := make([]types.LogClusterConfigItem, 0, len(rpcResp.Backends))
	for _, item := range rpcResp.Backends {
		items = append(items, types.LogClusterConfigItem{
			Id:                 item.Id,
			ClusterUuid:        item.ClusterUuid,
			AppName:            item.AppName,
			AppCode:            item.AppCode,
			AppType:            item.AppType,
			IsDefault:          item.IsDefault,
			AppUrl:             item.AppUrl,
			Port:               item.Port,
			Protocol:           item.Protocol,
			AuthEnabled:        item.AuthEnabled,
			AuthType:           item.AuthType,
			TlsEnabled:         item.TlsEnabled,
			InsecureSkipVerify: item.InsecureSkipVerify,
			Status:             item.Status,
			CreatedAt:          item.CreatedAt,
			UpdatedAt:          item.UpdatedAt,
		})
	}

	return &types.LogClusterConfigResponse{
		ClusterUuid:    rpcResp.ClusterUuid,
		DefaultBackend: rpcResp.DefaultBackend,
		Backends:       items,
	}, nil
}
