package managerservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesDeployLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesDeployLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesDeployLogic {
	return &AlertRulesDeployLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AlertRulesDeployLogic) AlertRulesDeploy(in *pb.DeployAlertRulesReq) (*pb.DeployAlertRulesResp, error) {
	if in.ClusterUuid == "" {
		return nil, errorx.Msg("集群UUID不能为空")
	}

	prometheusRule, err := l.buildPrometheusRule(in.FileId, in.Namespace, in.GroupIds, in.RuleIds, in.ClusterUuid)
	if err != nil {
		return nil, err
	}

	clusterClient, err := l.svcCtx.K8sManager.GetCluster(l.ctx, in.ClusterUuid)
	if err != nil {
		l.Errorf("获取集群客户端失败: %v", err)
		return nil, errorx.Msg("获取集群连接失败，请检查集群状态")
	}

	ruleOperator := clusterClient.PrometheusRule()
	if err := l.applyPrometheusRule(ruleOperator, prometheusRule); err != nil {
		l.Errorf("部署 PrometheusRule 失败: %v", err)
		return nil, errorx.Msg("部署告警规则失败")
	}

	yamlBytes, _ := yaml.Marshal(prometheusRule)
	l.Infof("成功部署 PrometheusRule: %s/%s", prometheusRule.Namespace, prometheusRule.Name)

	return &pb.DeployAlertRulesResp{
		YamlStr: string(yamlBytes),
	}, nil
}

func (l *AlertRulesDeployLogic) applyPrometheusRule(operator types.PrometheusRuleOperator, rule *monitoringv1.PrometheusRule) error {
	_, err := operator.Get(rule.Namespace, rule.Name)
	if err != nil {
		return operator.Create(rule)
	}
	return operator.Update(rule.Namespace, rule.Name, rule)
}

func (l *AlertRulesDeployLogic) buildPrometheusRule(fileId uint64, namespace string, groupIds []uint64, ruleIds []uint64, clusterUuid string) (*monitoringv1.PrometheusRule, error) {
	file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, fileId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	if namespace == "" {
		namespace = file.Namespace
	}
	if namespace == "" {
		namespace = "monitoring"
	}

	groups, err := l.fetchGroups(fileId, groupIds)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, errorx.Msg("没有可部署的分组")
	}

	fileLabels := l.parseLabels(file.Labels)
	if clusterUuid != "" {
		fileLabels["cluster_uuid"] = clusterUuid
	}

	ruleIdSet := make(map[uint64]bool)
	for _, id := range ruleIds {
		ruleIdSet[id] = true
	}

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

	for _, group := range groups {
		if group.IsEnabled != 1 {
			continue
		}

		ruleGroup := l.buildRuleGroup(group, ruleIdSet, clusterUuid)
		if len(ruleGroup.Rules) > 0 {
			prometheusRule.Spec.Groups = append(prometheusRule.Spec.Groups, ruleGroup)
		}
	}

	if len(prometheusRule.Spec.Groups) == 0 {
		return nil, errorx.Msg("没有可部署的规则")
	}

	return prometheusRule, nil
}

func (l *AlertRulesDeployLogic) fetchGroups(fileId uint64, groupIds []uint64) ([]*model.AlertRuleGroups, error) {
	if len(groupIds) == 0 {
		return l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "sort_order", true, "`file_id` = ?", fileId)
	}

	var groups []*model.AlertRuleGroups
	for _, groupId := range groupIds {
		group, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, groupId)
		if err != nil {
			l.Errorf("查询分组失败: %d, error: %v", groupId, err)
			continue
		}
		if group.FileId != fileId {
			l.Errorf("分组 %d 不属于文件 %d", groupId, fileId)
			continue
		}
		groups = append(groups, group)
	}
	return groups, nil
}

func (l *AlertRulesDeployLogic) buildRuleGroup(group *model.AlertRuleGroups, ruleIdSet map[uint64]bool, clusterUuid string) monitoringv1.RuleGroup {
	interval := monitoringv1.Duration(group.Interval)
	ruleGroup := monitoringv1.RuleGroup{
		Name:     group.GroupName,
		Interval: &interval,
		Rules:    []monitoringv1.Rule{},
	}

	rules, err := l.svcCtx.AlertRulesModel.SearchNoPage(l.ctx, "sort_order", true, "`group_id` = ?", group.Id)
	if err != nil {
		l.Errorf("查询规则列表失败: group_id=%d, error: %v", group.Id, err)
		return ruleGroup
	}

	for _, rule := range rules {
		if len(ruleIdSet) > 0 && !ruleIdSet[rule.Id] {
			continue
		}
		if rule.IsEnabled != 1 {
			continue
		}

		ruleGroup.Rules = append(ruleGroup.Rules, l.buildRule(rule, clusterUuid))
	}

	return ruleGroup
}

func (l *AlertRulesDeployLogic) buildRule(rule *model.AlertRules, clusterUuid string) monitoringv1.Rule {
	labels := l.parseLabels(rule.Labels)
	labels["severity"] = rule.Severity
	if clusterUuid != "" {
		labels["cluster_uuid"] = clusterUuid
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

func (l *AlertRulesDeployLogic) parseLabels(sqlStr sql.NullString) map[string]string {
	if !sqlStr.Valid || sqlStr.String == "" {
		return map[string]string{}
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(sqlStr.String), &result); err != nil {
		return map[string]string{}
	}
	return result
}

// GeneratePrometheusRuleYaml 生成 PrometheusRule YAML 字符串（供预览使用）
func (l *AlertRulesDeployLogic) GeneratePrometheusRuleYaml(fileId uint64, namespace string, groupIds []uint64, ruleIds []uint64, clusterUuid string) (string, error) {
	prometheusRule, err := l.buildPrometheusRule(fileId, namespace, groupIds, ruleIds, clusterUuid)
	if err != nil {
		return "", err
	}

	yamlBytes, err := yaml.Marshal(prometheusRule)
	if err != nil {
		l.Errorf("序列化 YAML 失败: %v", err)
		return "", errorx.Msg("生成 YAML 失败")
	}

	return string(yamlBytes), nil
}
