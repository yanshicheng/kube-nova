package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapGetDataLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ConfigMap 数据（key-value 列表）
func NewConfigMapGetDataLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapGetDataLogic {
	return &ConfigMapGetDataLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapGetDataLogic) ConfigMapGetData(req *types.DefaultNameRequest) (resp *types.GetConfigMapDataResponse, err error) {
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	configMapClient := client.ConfigMaps()

	configMapData, err := configMapClient.GetData(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 ConfigMap 数据失败: %v", err)
		return nil, fmt.Errorf("获取 ConfigMap 数据失败")
	}

	items := make([]types.ConfigMapDataItem, 0, len(configMapData.Data))
	for _, item := range configMapData.Data {
		items = append(items, types.ConfigMapDataItem{
			Key:   item.Key,
			Value: item.Value,
		})
	}

	l.Infof("成功获取 ConfigMap 数据: %s, 共 %d 项", req.Name, len(items))
	return &types.GetConfigMapDataResponse{
		Name: configMapData.Name,
		Data: items,
	}, nil
}
