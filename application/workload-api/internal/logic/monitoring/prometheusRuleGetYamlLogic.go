package monitoring

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PrometheusRuleGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PrometheusRule YAML
func NewPrometheusRuleGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrometheusRuleGetYamlLogic {
	return &PrometheusRuleGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PrometheusRuleGetYamlLogic) PrometheusRuleGetYaml(req *types.MonitoringResourceNameRequest) (resp string, err error) {
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

	// 获取 PrometheusRule 操作器
	ruleOp := client.PrometheusRule()

	// 调用 Get 方法获取 YAML
	yamlStr, err := ruleOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 PrometheusRule YAML 失败: %v", err)
		return "", fmt.Errorf("获取 PrometheusRule YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 PrometheusRule %s/%s 的 YAML", username, req.Namespace, req.Name)

	return yamlStr, nil
}
