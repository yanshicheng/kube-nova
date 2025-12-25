package core

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/client/managerservice"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

type IngressGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Ingress 详情
func NewIngressGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressGetLogic {
	return &IngressGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressGetLogic) IngressGet(req *types.DefaultNameRequest) (resp *types.IngressDetail, err error) {
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

	ingressClient := client.Ingresses()

	detail, err := ingressClient.GetDetail(workloadInfo.Data.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Ingress 详情失败: %v", err)
		return nil, fmt.Errorf("获取 Ingress 详情失败")
	}

	resp = &types.IngressDetail{
		Name:              detail.Name,
		Namespace:         detail.Namespace,
		IngressClass:      detail.IngressClass,
		Rules:             convertRulesToTypes(detail.Rules),
		TLS:               convertTLSToTypes(detail.TLS),
		Labels:            detail.Labels,
		Annotations:       detail.Annotations,
		Age:               detail.Age,
		CreationTimestamp: detail.CreationTimestamp,
	}

	// 转换 DefaultBackend
	if detail.DefaultBackend != nil {
		backend := convertBackendToTypes(*detail.DefaultBackend)
		resp.DefaultBackend = backend
	}

	// 转换 LoadBalancer
	resp.LoadBalancer = convertLoadBalancerToTypes(detail.LoadBalancer)

	l.Infof("成功获取 Ingress 详情: %s/%s", workloadInfo.Data.Namespace, req.Name)
	return resp, nil
}
