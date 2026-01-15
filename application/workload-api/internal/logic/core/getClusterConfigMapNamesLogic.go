package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterConfigMapNamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定集群指定命名空间的 ConfigMap 名称列表
func NewGetClusterConfigMapNamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterConfigMapNamesLogic {
	return &GetClusterConfigMapNamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterConfigMapNamesLogic) GetClusterConfigMapNames(req *types.ClusterResourceNamesRequest) (resp []string, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	configMapClient := client.ConfigMaps()

	listResp, err := configMapClient.List(req.Namespace, "", "")
	if err != nil {
		l.Errorf("获取 ConfigMap 列表失败: %v", err)
		return nil, fmt.Errorf("获取 ConfigMap 列表失败")
	}

	// 提取所有 ConfigMap 名称
	names := make([]string, 0, len(listResp.Items))
	for _, item := range listResp.Items {
		names = append(names, item.Name)
	}

	l.Infof("成功获取集群 %s 命名空间 %s 的 ConfigMap 名称列表，共 %d 个", req.ClusterUuid, req.Namespace, len(names))
	return names, nil
}
