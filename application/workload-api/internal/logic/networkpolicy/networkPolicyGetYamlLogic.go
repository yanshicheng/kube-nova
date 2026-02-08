package networkpolicy

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type NetworkPolicyGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 NetworkPolicy YAML
func NewNetworkPolicyGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *NetworkPolicyGetYamlLogic {
	return &NetworkPolicyGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *NetworkPolicyGetYamlLogic) NetworkPolicyGetYaml(req *types.NetworkPolicyNameRequest) (resp string, err error) {
	username, ok := l.ctx.Value("username").(string)
	if !ok {
		username = "system"
	}

	// 获取集群客户端
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return "", fmt.Errorf("获取集群客户端失败")
	}

	// 获取 NetworkPolicy 操作器
	networkPolicyOp := client.NetworkPolicies()

	// 调用 GetYaml 方法获取 YAML
	yamlStr, err := networkPolicyOp.GetYaml(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 NetworkPolicy YAML 失败: %v", err)
		return "", fmt.Errorf("获取 NetworkPolicy YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 NetworkPolicy %s/%s 的 YAML", username, req.Namespace, req.Name)

	return yamlStr, nil
}
