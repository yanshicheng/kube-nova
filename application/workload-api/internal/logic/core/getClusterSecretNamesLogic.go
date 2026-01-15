package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetClusterSecretNamesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取指定集群指定命名空间的 Secret 名称列表
func NewGetClusterSecretNamesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetClusterSecretNamesLogic {
	return &GetClusterSecretNamesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetClusterSecretNamesLogic) GetClusterSecretNames(req *types.ClusterResourceNamesRequest) (resp []string, err error) {
	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 Secret 客户端
	secretClient := client.Secrets()

	// 获取指定命名空间的 Secret 列表
	listResp, err := secretClient.List(req.Namespace, "", "", "")
	if err != nil {
		l.Errorf("获取 Secret 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Secret 列表失败")
	}

	// 提取所有 Secret 名称
	names := make([]string, 0, len(listResp.Items))
	for _, item := range listResp.Items {
		names = append(names, item.Name)
	}

	l.Infof("成功获取集群 %s 命名空间 %s 的 Secret 名称列表，共 %d 个", req.ClusterUuid, req.Namespace, len(names))
	return names, nil
}
