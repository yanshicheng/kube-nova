package networkpolicy

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NetworkPolicyGetAffectedPodsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取受 NetworkPolicy 影响的 Pod
func NewNetworkPolicyGetAffectedPodsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyGetAffectedPodsLogic {
	return &NetworkPolicyGetAffectedPodsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyGetAffectedPodsLogic) NetworkPolicyGetAffectedPods(req *types.NetworkPolicyNameRequest) (resp *types.NetworkPolicyAffectedPodsResponse, err error) {
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

	// 调用 GetAffectedPods 方法
	affectedPodsResp, err := networkPolicyOp.GetAffectedPods(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取受影响的 Pod 失败: %v", err)
		return nil, fmt.Errorf("获取受影响的 Pod 失败: %v", err)
	}

	// 转换为 API 响应格式
	affectedPods := make([]types.NetworkPolicyAffectedPod, len(affectedPodsResp.AffectedPods))
	for i, pod := range affectedPodsResp.AffectedPods {
		affectedPods[i] = types.NetworkPolicyAffectedPod{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
			Matched:   pod.Matched,
			Reason:    pod.Reason,
		}
	}

	response := &types.NetworkPolicyAffectedPodsResponse{
		NetworkPolicyName: affectedPodsResp.NetworkPolicyName,
		NetworkPolicyNS:   affectedPodsResp.NetworkPolicyNS,
		AffectedPods:      affectedPods,
		TotalAffected:     affectedPodsResp.TotalAffected,
	}

	l.Infof("用户: %s, 成功获取 NetworkPolicy %s/%s 受影响的 Pod，共 %d 个",
		username, req.Namespace, req.Name, response.TotalAffected)

	return response, nil
}
