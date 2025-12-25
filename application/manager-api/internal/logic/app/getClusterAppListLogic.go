package app

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-api/internal/types"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterAppListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定集群下的所有应用配置列表
func NewGetClusterAppListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterAppListLogic {
	return &GetClusterAppListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterAppListLogic) GetClusterAppList(req *types.ClusterAppListRequest) (resp []types.ClusterAppDetail, err error) {
	// 3. 构建RPC请求
	rpcReq := &managerservice.ClusterAppListReq{
		ClusterUuid: req.ClusterUuid,
	}

	rpcResp, err := l.svcCtx.ManagerRpc.AppOnClusterList(l.ctx, rpcReq)
	if err != nil {
		l.Errorf("调用RPC服务获取应用列表失败，集群UUID: %s, error: %v", req.ClusterUuid, err)
		return nil, fmt.Errorf("获取应用列表失败: %w", err)
	}

	// 5. 转换响应数据
	appList := make([]types.ClusterAppDetail, 0, len(rpcResp.List))

	for _, app := range rpcResp.List {
		// 转换单个应用详情
		appDetail := types.ClusterAppDetail{
			Id:                 uint64(app.Id),
			ClusterUuid:        app.ClusterUuid,
			AppName:            app.AppName,
			AppCode:            app.AppCode,
			AppType:            app.AppType,
			IsDefault:          app.IsDefault,
			AppUrl:             app.AppUrl,
			Port:               app.Port,
			Protocol:           app.Protocol,
			AuthEnabled:        app.AuthEnabled,
			AuthType:           app.AuthType,
			Username:           app.Username, // 列表不返回敏感信息
			Password:           app.Password, // 列表不返回敏感信息
			Token:              app.Token,    // 列表不返回敏感信息
			AccessKey:          app.AccessKey,
			AccessSecret:       app.AccessSecret, // 列表不返回敏感信息
			TlsEnabled:         app.TlsEnabled,
			CaFile:             app.CaFile,     // 列表不返回证书内容
			CaKey:              app.CaKey,      // 列表不返回密钥
			CaCert:             app.CaCert,     // 列表不返回证书内容
			ClientCert:         app.ClientCert, // 列表不返回证书内容
			ClientKey:          app.ClientKey,  // 列表不返回密钥
			InsecureSkipVerify: app.InsecureSkipVerify,
			Status:             app.Status,
			CreatedBy:          app.CreatedBy,
			UpdatedBy:          app.UpdatedBy,
			CreatedAt:          app.CreatedAt,
			UpdatedAt:          app.UpdatedAt,
		}

		appList = append(appList, appDetail)
	}

	return appList, nil
}
