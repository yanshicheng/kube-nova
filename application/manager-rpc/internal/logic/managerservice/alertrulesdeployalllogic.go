// alertrulesdeployalllogic.go
package managerservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	types2 "github.com/yanshicheng/kube-nova/common/k8smanager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesDeployAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesDeployAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesDeployAllLogic {
	return &AlertRulesDeployAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertRulesDeployAllLogic) AlertRulesDeployAll(in *pb.DeployAllAlertRulesReq) (*pb.DeployAllAlertRulesResp, error) {
	if in.ClusterUuid == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}
	if in.Namespace == "" {
		return nil, errorx.Msg("命名空间不能为空")
	}

	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, in.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, errorx.Msg("获取集群连接失败，请检查集群状态")
	}

	ruleOperator := clusterClient.PrometheusRule()

	files, err := l.svcCtx.AlertRuleFilesModel.SearchNoPage(l.ctx, "sort_order", true, "`is_enabled` = ? AND `is_deleted` = ?", 1, 0)
	if err != nil {
		l.Errorf("查询告警规则文件列表失败: %v", err)
		return nil, errorx.Msg("查询告警规则文件列表失败")
	}

	if len(files) == 0 {
		return nil, errorx.Msg("没有可部署的告警规则文件")
	}

	var fileCount, groupCount, ruleCount int64

	for _, file := range files {
		prometheusRule, gCount, rCount := l.buildPrometheusRuleForFile(file, in.Namespace)
		if prometheusRule == nil || len(prometheusRule.Spec.Groups) == 0 {
			continue
		}

		if err := l.applyPrometheusRule(ruleOperator, prometheusRule); err != nil {
			l.Errorf("部署文件 %s 到集群失败: %v", file.FileCode, err)
			continue
		}

		fileCount++
		groupCount += gCount
		ruleCount += rCount
		l.Infof("成功部署告警规则文件: %s, 分组数: %d, 规则数: %d", file.FileCode, gCount, rCount)
	}

	if fileCount == 0 {
		return nil, errorx.Msg("没有成功部署的告警规则")
	}

	return &pb.DeployAllAlertRulesResp{
		FileCount:  fileCount,
		GroupCount: groupCount,
		RuleCount:  ruleCount,
	}, nil
}

func (l *AlertRulesDeployAllLogic) applyPrometheusRule(operator types2.PrometheusRuleOperator, rule *monitoringv1.PrometheusRule) error {
	_, err := operator.Get(rule.Namespace, rule.Name)
	if err != nil {
		return operator.Create(rule)
	}
	return operator.Update(rule.Namespace, rule.Name, rule)
}

func (l *AlertRulesDeployAllLogic) buildPrometheusRuleForFile(file *model.AlertRuleFiles, namespace string) (*monitoringv1.PrometheusRule, int64, int64) {
	groups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "sort_order", true, "`file_id` = ? AND `is_enabled` = ? AND `is_deleted` = ?", file.Id, 1, 0)
	if err != nil || len(groups) == 0 {
		return nil, 0, 0
	}

	fileLabels := l.parseLabels(file.Labels)

	prometheusRule := &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      file.FileCode,
			Namespace: namespace,
			Labels:    fileLabels,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}

	var groupCount, ruleCount int64

	for _, group := range groups {
		ruleGroup, count := l.buildRuleGroup(group)
		if len(ruleGroup.Rules) > 0 {
			prometheusRule.Spec.Groups = append(prometheusRule.Spec.Groups, ruleGroup)
			groupCount++
			ruleCount += count
		}
	}

	return prometheusRule, groupCount, ruleCount
}

func (l *AlertRulesDeployAllLogic) buildRuleGroup(group *model.AlertRuleGroups) (monitoringv1.RuleGroup, int64) {
	interval := monitoringv1.Duration(group.Interval)
	ruleGroup := monitoringv1.RuleGroup{
		Name:     group.GroupName,
		Interval: &interval,
		Rules:    []monitoringv1.Rule{},
	}

	rules, err := l.svcCtx.AlertRulesModel.SearchNoPage(l.ctx, "sort_order", true, "`group_id` = ? AND `is_enabled` = ? AND `is_deleted` = ?", group.Id, 1, 0)
	if err != nil {
		l.Errorf("查询规则列表失败: group_id=%d, error: %v", group.Id, err)
		return ruleGroup, 0
	}

	for _, rule := range rules {
		ruleGroup.Rules = append(ruleGroup.Rules, l.buildRule(rule))
	}

	return ruleGroup, int64(len(ruleGroup.Rules))
}

func (l *AlertRulesDeployAllLogic) buildRule(rule *model.AlertRules) monitoringv1.Rule {
	labels := l.parseLabels(rule.Labels)
	if rule.Severity != "" {
		labels["severity"] = rule.Severity
	}

	annotations := l.parseLabels(rule.Annotations)
	if rule.Summary != "" {
		annotations["summary"] = rule.Summary
	}
	if rule.Description.Valid && rule.Description.String != "" {
		annotations["description"] = rule.Description.String
	}
	if _, ok := annotations["value"]; !ok {
		annotations["value"] = "{{ $value }}"
	}

	var forDuration *monitoringv1.Duration
	if rule.ForDuration != "" {
		duration := monitoringv1.Duration(rule.ForDuration)
		forDuration = &duration
	}

	return monitoringv1.Rule{
		Alert:       rule.AlertName,
		Expr:        intstr.FromString(rule.Expr),
		For:         forDuration,
		Labels:      labels,
		Annotations: annotations,
	}
}

func (l *AlertRulesDeployAllLogic) parseLabels(sqlStr sql.NullString) map[string]string {
	if !sqlStr.Valid || sqlStr.String == "" {
		return map[string]string{
			"prometheus": "k8s",
			"role":       "alert-rules",
		}
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(sqlStr.String), &result); err != nil {
		return map[string]string{
			"prometheus": "k8s",
			"role":       "alert-rules",
		}
	}
	return result
}
