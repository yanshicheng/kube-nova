package cluster

import (
	"context"
	"fmt"

	"github.com/yanshicheng/kube-nova/application/workload-api/internal/svc"
	"github.com/yanshicheng/kube-nova/application/workload-api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ClusterRoleGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 获取 ClusterRole 详情
func NewClusterRoleGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClusterRoleGetLogic {
	return &ClusterRoleGetLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ClusterRoleGetLogic) ClusterRoleGet(req *types.ClusterResourceNameRequest) (resp *types.ClusterRoleDetail, err error) {
	client, err := l.svcCtx.K8sManager.GetCluster(l.ctx, req.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, fmt.Errorf("获取集群客户端失败")
	}

	crOp := client.ClusterRoles()
	cr, err := crOp.Get(req.Name)
	if err != nil {
		l.Errorf("获取 ClusterRole 详情失败: %v", err)
		return nil, fmt.Errorf("获取 ClusterRole 详情失败")
	}

	resp = &types.ClusterRoleDetail{
		Name:              cr.Name,
		Rules:             make([]types.PolicyRuleInfo, 0, len(cr.Rules)),
		Labels:            cr.Labels,
		Annotations:       cr.Annotations,
		Age:               formatAge(cr.CreationTimestamp.Time),
		CreationTimestamp: cr.CreationTimestamp.UnixMilli(),
	}

	// 转换 Rules
	for _, rule := range cr.Rules {
		resp.Rules = append(resp.Rules, types.PolicyRuleInfo{
			Verbs:           rule.Verbs,
			APIGroups:       rule.APIGroups,
			Resources:       rule.Resources,
			ResourceNames:   rule.ResourceNames,
			NonResourceURLs: rule.NonResourceURLs,
		})
	}

	// 转换 AggregationRule
	if cr.AggregationRule != nil {
		resp.AggregationRule = types.AggregationRuleInfo{
			ClusterRoleSelectors: make([]types.LabelSelectorInfo, 0, len(cr.AggregationRule.ClusterRoleSelectors)),
		}
		for _, selector := range cr.AggregationRule.ClusterRoleSelectors {
			selectorInfo := types.LabelSelectorInfo{
				MatchLabels: selector.MatchLabels,
			}
			if len(selector.MatchExpressions) > 0 {
				selectorInfo.MatchExpressions = make([]types.LabelSelectorExpression, 0, len(selector.MatchExpressions))
				for _, expr := range selector.MatchExpressions {
					selectorInfo.MatchExpressions = append(selectorInfo.MatchExpressions, types.LabelSelectorExpression{
						Key:      expr.Key,
						Operator: string(expr.Operator),
						Values:   expr.Values,
					})
				}
			}
			resp.AggregationRule.ClusterRoleSelectors = append(resp.AggregationRule.ClusterRoleSelectors, selectorInfo)
		}
	}

	return resp, nil
}
