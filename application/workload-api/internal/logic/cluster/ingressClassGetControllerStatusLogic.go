package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type IngressClassGetControllerStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 IngressClass 控制器状态
func NewIngressClassGetControllerStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *IngressClassGetControllerStatusLogic {
	return &IngressClassGetControllerStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *IngressClassGetControllerStatusLogic) IngressClassGetControllerStatus(req *types.IngressClassNameRequest) (resp *types.IngressClassControllerStatus, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	icOp := client.IngressClasses()
	result, err := icOp.GetControllerStatus(req.Name)
	if err != nil {
		l.Errorf("获取 IngressClass 控制器状态失败: %v", err)
		return nil, fmt.Errorf("获取 IngressClass 控制器状态失败")
	}

	return &types.IngressClassControllerStatus{
		IngressClassName:   result.IngressClassName,
		ControllerName:     result.ControllerName,
		ControllerPods:     result.ControllerPods,
		ControllerReady:    result.ControllerReady,
		ControllerReplicas: result.ControllerReplicas,
		Namespace:          result.Namespace,
	}, nil
}
