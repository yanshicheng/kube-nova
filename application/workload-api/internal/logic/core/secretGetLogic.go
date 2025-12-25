package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type SecretGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Secret 详情
func NewSecretGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SecretGetLogic {
	return &SecretGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SecretGet 获取 Secret 详情
func (l *SecretGetLogic) SecretGet(req *types.DefaultNameRequest) (resp *types.SecretDetail, err error) {
	workloadInfo, err := l.svcCtx.ManagerRpc.ProjectWorkspaceGetById(l.ctx, &managerservice.GetOnecProjectWorkspaceByIdReq{Id: req.WorkloadId})
	if err != nil {
		l.Errorf("获取项目工作空间详情失败: %v", err)
		return nil, fmt.Errorf("获取项目工作空间详情失败")
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, workloadInfo.Data.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	// 初始化 secret 客户端
	secretClient := client.Secrets()

	// 获取 secret 详情
	secret, err := secretClient.Get(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 secret 详情失败: %v", err)
		return nil, fmt.Errorf("获取 secret 详情失败")
	}
	resp = &types.SecretDetail{
		Name:              secret.Name,
		Namespace:         secret.Namespace,
		Age:               secret.CreationTimestamp.Format("2006-01-02 15:04:05"),
		CreationTimestamp: secret.CreationTimestamp.Unix(),
		Data:              secret.Data,
		Labels:            secret.Labels,
		Annotations:       secret.Annotations,
		Type:              string(secret.Type),
	}
	return
}
