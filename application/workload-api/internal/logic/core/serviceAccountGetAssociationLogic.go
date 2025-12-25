// serviceAccountGetAssociationLogic.go
package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ServiceAccountGetAssociationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewServiceAccountGetAssociationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ServiceAccountGetAssociationLogic {
	return &ServiceAccountGetAssociationLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ServiceAccountGetAssociationLogic) ServiceAccountGetAssociation(req *types.ClusterNamespaceResourceRequest) (resp *types.ServiceAccountAssociation, err error) {
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

	// 获取关联信息
	association, err := saOp.GetAssociation(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 ServiceAccount 关联信息失败: %v", err)
		return nil, fmt.Errorf("获取 ServiceAccount 关联信息失败")
	}
	// 转换响应格式
	resp = &types.ServiceAccountAssociation{
		ServiceAccountName: association.ServiceAccountName,
		Secrets:            association.Secrets,
		ImagePullSecrets:   association.ImagePullSecrets,
		Pods:               association.Pods,
		PodCount:           len(association.Pods),
		Namespace:          req.Namespace,
	}

	l.Infof("用户: %s, 成功获取 ServiceAccount 关联信息: %s/%s", username, req.Namespace, req.Name)
	return resp, nil
}
