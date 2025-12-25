package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassGetDefaultLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取默认 IngressClass
func NewIngressClassGetDefaultLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassGetDefaultLogic {
	return &IngressClassGetDefaultLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassGetDefaultLogic) IngressClassGetDefault(req *types.ClusterResourceListRequest) (resp *types.IngressClassDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	ic, err := icOp.GetDefaultIngressClass()
	if err != nil {
		l.Errorf("获取默认 IngressClass 失败: %v", err)
		return nil, fmt.Errorf("获取默认 IngressClass 失败")
	}

	resp = &types.IngressClassDetail{
		Name:              ic.Name,
		Controller:        ic.Spec.Controller,
		IsDefault:         true,
		Labels:            ic.Labels,
		Annotations:       ic.Annotations,
		Age:               formatAge(ic.CreationTimestamp.Time),
		CreationTimestamp: ic.CreationTimestamp.UnixMilli(),
	}

	// 处理 Parameters
	if ic.Spec.Parameters != nil {
		resp.Parameters = types.IngressClassParameters{
			Kind: ic.Spec.Parameters.Kind,
			Name: ic.Spec.Parameters.Name,
		}
		if ic.Spec.Parameters.APIGroup != nil {
			resp.Parameters.APIGroup = *ic.Spec.Parameters.APIGroup
		}
		if ic.Spec.Parameters.Namespace != nil {
			resp.Parameters.Namespace = *ic.Spec.Parameters.Namespace
		}
		if ic.Spec.Parameters.Scope != nil {
			resp.Parameters.Scope = *ic.Spec.Parameters.Scope
		}
	}

	return resp, nil
}
