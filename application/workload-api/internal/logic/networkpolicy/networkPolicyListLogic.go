package networkpolicy

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NetworkPolicyListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 NetworkPolicy 列表
func NewNetworkPolicyListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyListLogic {
	return &NetworkPolicyListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyListLogic) NetworkPolicyList(req *types.NetworkPolicyListRequest) (resp *types.NetworkPolicyListResponse, err error) {
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

	// 获取 NetworkPolicy 操作器
	networkPolicyOp := client.NetworkPolicies()

	// 调用 List 方法
	npList, err := networkPolicyOp.List(req.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 NetworkPolicy 列表失败: %v", err)
		return nil, fmt.Errorf("获取 NetworkPolicy 列表失败")
	}

	// 转换为 API 响应格式
	items := make([]types.NetworkPolicyListItem, len(npList.Items))
	for i, item := range npList.Items {
		items[i] = types.NetworkPolicyListItem{
			Name:              item.Name,
			Namespace:         item.Namespace,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
		}
	}

	l.Infof("用户: %s, 成功获取集群 %s 命名空间 %s 的 NetworkPolicy 列表，共 %d 个",
		username, req.ClusterUuid, req.Namespace, npList.Total)

	return &types.NetworkPolicyListResponse{
		Total: npList.Total,
		Items: items,
	}, nil
}
