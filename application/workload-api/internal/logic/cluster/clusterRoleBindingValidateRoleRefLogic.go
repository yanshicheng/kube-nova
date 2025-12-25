package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleBindingValidateRoleRefLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 验证角色引用是否有效
func NewClusterRoleBindingValidateRoleRefLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleBindingValidateRoleRefLogic {
	return &ClusterRoleBindingValidateRoleRefLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleBindingValidateRoleRefLogic) ClusterRoleBindingValidateRoleRef(req *types.ClusterResourceNameRequest) (resp *types.ValidateRoleRefResponse, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crbOp := client.ClusterRoleBindings()
	valid, message, err := crbOp.ValidateRoleRef(req.Name)
	if err != nil {
		l.Errorf("验证角色引用失败: %v", err)
		return nil, fmt.Errorf("验证角色引用失败")
	}

	return &types.ValidateRoleRefResponse{
		Valid:   valid,
		Message: message,
	}, nil
}
