package monitoring

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ProbeGetYamlLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 Probe YAML
func NewProbeGetYamlLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProbeGetYamlLogic {
	return &ProbeGetYamlLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ProbeGetYamlLogic) ProbeGetYaml(req *types.MonitoringResourceNameRequest) (resp string, err error) {
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

	// 获取 Probe 操作器
	probeOp := client.Probe()

	// 调用 Get 方法获取 YAML
	yamlStr, err := probeOp.Get(req.Namespace, req.Name)
	if err != nil {
		l.Errorf("获取 Probe YAML 失败: %v", err)
		return "", fmt.Errorf("获取 Probe YAML 失败")
	}

	l.Infof("用户: %s, 成功获取 Probe %s/%s 的 YAML", username, req.Namespace, req.Name)

	return yamlStr, nil
}
