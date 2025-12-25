package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleBindingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleBindingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleBindingListLogic {
	return &RoleBindingListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleBindingListLogic) RoleBindingList(req *types.ClusterNamespaceRequest) (resp *types.RoleBindingListResponse, err error) {
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

	// 获取 RoleBinding operator
	rbOp := client.RoleBindings()

	// 获取列表
	rbList, err := rbOp.List(req.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 RoleBinding 列表失败: %v", err)
		return nil, fmt.Errorf("获取 RoleBinding 列表失败")
	}

	// 转换响应格式
	items := make([]types.RoleBindingListItem, 0, len(rbList.Items))
	for _, rb := range rbList.Items {
		// 构建 Role 字段（格式：Role/rolename 或 ClusterRole/clusterrolename）
		roleStr := fmt.Sprintf("%s/%s", rb.Role, rb.Role)
		// 如果 types 中 RoleBindingInfoo 结构包含单独的 Kind 和 Name，需要相应调整

		items = append(items, types.RoleBindingListItem{
			Name:              rb.Name,
			Namespace:         rb.Namespace,
			Role:              roleStr,
			Age:               rb.Age,
			Labels:            rb.Labels,
			Annotations:       rb.Annotations,
			CreationTimestamp: rb.CreationTimestamp,
		})
	}

	l.Infof("用户: %s, 成功获取 RoleBinding 列表，共 %d 个", username, len(items))
	return &types.RoleBindingListResponse{
		Total: rbList.Total,
		Items: items,
	}, nil
}
