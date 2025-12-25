package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassDescribeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 IngressClass Describe
func NewIngressClassDescribeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassDescribeLogic {
	return &IngressClassDescribeLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassDescribeLogic) IngressClassDescribe(req *types.IngressClassNameRequest) (resp string, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	descStr, err := icOp.Describe(req.Name)
	if err != nil {
		l.Errorf("获取 IngressClass Describe 失败: %v", err)
		return "", fmt.Errorf("获取 IngressClass Describe 失败")
	}

	return descStr, nil
}
