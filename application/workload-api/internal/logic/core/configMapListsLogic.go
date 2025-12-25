package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapListsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ConfigMap 列表
func NewConfigMapListsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapListsLogic {
	return &ConfigMapListsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapListsLogic) ConfigMapLists(req *types.ListRequest) (resp *types.ConfigMapListResponse, err error) {
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

	configMapList, err := configMapClient.List(workloadInfo.Data.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 ConfigMap 列表失败: %v", err)
		return nil, fmt.Errorf("获取 ConfigMap 列表失败")
	}

	items := make([]types.ConfigMapListItem, 0, len(configMapList.Items))
	for _, cm := range configMapList.Items {
		items = append(items, types.ConfigMapListItem{
			Name:              cm.Name,
			Namespace:         cm.Namespace,
			DataCount:         cm.DataCount,
			Age:               cm.Age,
			CreationTimestamp: cm.CreationTimestamp,
		})
	}

	l.Infof("成功获取 ConfigMap 列表，共 %d 个", len(items))
	return &types.ConfigMapListResponse{
		Total: configMapList.Total,
		Items: items,
	}, nil
}
