package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceAccountListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountListLogic {
	return &ServiceAccountListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountListLogic) ServiceAccountList(req *types.ClusterNamespaceRequest) (resp *types.ServiceAccountListResponse, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 获取 ServiceAccount operator
	saOp := client.ServiceAccounts()

	// 获取列表
	saList, err := saOp.List(req.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 ServiceAccount 列表失败: %v", err)
		return nil, fmt.Errorf("获取 ServiceAccount 列表失败")
	}

	// 转换响应格式
	items := make([]types.ServiceAccountListItem, 0, len(saList.Items))
	for _, sa := range saList.Items {
		items = append(items, types.ServiceAccountListItem{
			Name:              sa.Name,
			Namespace:         sa.Namespace,
			Secrets:           sa.Secrets,
			Age:               sa.Age,
			CreationTimestamp: sa.CreationTimestamp,
		})
	}

	l.Infof("用户: %s, 成功获取 ServiceAccount 列表，共 %d 个", username, len(items))
	return &types.ServiceAccountListResponse{
		Total: saList.Total,
		Items: items,
	}, nil
}
