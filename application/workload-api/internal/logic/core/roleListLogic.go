package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RoleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RoleListLogic {
	return &RoleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RoleListLogic) RoleList(req *types.ClusterNamespaceRequest) (resp *types.RoleListResponse, err error) {
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

	// 获取 Role operator
	roleOp := client.Roles()

	// 获取列表
	roleList, err := roleOp.List(req.Namespace, req.Search, req.LabelSelector)
	if err != nil {
		l.Errorf("获取 Role 列表失败: %v", err)
		return nil, fmt.Errorf("获取 Role 列表失败")
	}

	// 转换响应格式
	items := make([]types.RoleListItem, 0, len(roleList.Items))
	for _, role := range roleList.Items {
		items = append(items, types.RoleListItem{
			Name:              role.Name,
			Namespace:         role.Namespace,
			Age:               role.Age,
			Labels:            role.Labels,
			Annotations:       role.Annotations,
			CreationTimestamp: role.CreationTimestamp,
		})
	}

	l.Infof("用户: %s, 成功获取 Role 列表，共 %d 个", username, len(items))
	return &types.RoleListResponse{
		Total: roleList.Total,
		Items: items,
	}, nil
}
