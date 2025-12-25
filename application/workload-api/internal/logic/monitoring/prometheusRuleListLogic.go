package monitoring

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type PrometheusRuleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 PrometheusRule 列表
func NewPrometheusRuleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrometheusRuleListLogic {
	return &PrometheusRuleListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *PrometheusRuleListLogic) PrometheusRuleList(req *types.MonitoringResourceListRequest) (resp *types.PrometheusRuleListResponse, err error) {
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

	// 获取 PrometheusRule 操作器
	ruleOp := client.PrometheusRule()

	// 调用 List 方法
	ruleList, err := ruleOp.List(req.Namespace, req.Search)
	if err != nil {
		l.Errorf("获取 PrometheusRule 列表失败: %v", err)
		return nil, fmt.Errorf("获取 PrometheusRule 列表失败")
	}

	// 转换为 API 响应格式
	items := make([]types.PrometheusRuleListItem, len(ruleList.Items))
	for i, item := range ruleList.Items {
		items[i] = types.PrometheusRuleListItem{
			Name:              item.Name,
			Namespace:         item.Namespace,
			Age:               item.Age,
			CreationTimestamp: item.CreationTimestamp,
			Labels:            item.Labels,
			Annotations:       item.Annotations,
		}
	}

	l.Infof("用户: %s, 成功获取集群 %s 命名空间 %s 的 PrometheusRule 列表，共 %d 个",
		username, req.ClusterUuid, req.Namespace, ruleList.Total)

	return &types.PrometheusRuleListResponse{
		Total: ruleList.Total,
		Items: items,
	}, nil
}
