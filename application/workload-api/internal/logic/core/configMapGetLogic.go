package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type ConfigMapGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ConfigMap 详情
func NewConfigMapGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConfigMapGetLogic {
	return &ConfigMapGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ConfigMapGetLogic) ConfigMapGet(req *types.DefaultNameRequest) (resp *types.ConfigMapDetail, err error) {
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

	configMap, err := configMapClient.Get(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 ConfigMap 详情失败: %v", err)
		return nil, fmt.Errorf("获取 ConfigMap 详情失败")
	}

	resp = &types.ConfigMapDetail{
		Name:              configMap.Name,
		Data:              configMap.Data,
		Labels:            configMap.Labels,
		Annotations:       configMap.Annotations,
		Age:               configMap.CreationTimestamp.Format("2006-01-02 15:04:05"),
		CreationTimestamp: configMap.CreationTimestamp.Unix(),
	}

	l.Infof("成功获取 ConfigMap 详情: %s", req.Name)
	return resp, nil
}
