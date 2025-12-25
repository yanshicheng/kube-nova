package managerservicelogic

import (
	"context"
	"encoding/json"
	"errors"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesBatchExportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesBatchExportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesBatchExportLogic {
	return &AlertRulesBatchExportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRulesBatchExport 批量导出 YAML (file级别 + 可选groups)
func (l *AlertRulesBatchExportLogic) AlertRulesBatchExport(in *pb.BatchExportAlertRulesReq) (*pb.BatchExportAlertRulesResp, error) {
	// 查询告警规则文件
	file, err := l.svcCtx.AlertRuleFilesModel.FindOne(l.ctx, in.FileId)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, errorx.Msg("告警规则文件不存在")
		}
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	// 查询要导出的分组
	var groups []*model.AlertRuleGroups
	if len(in.GroupIds) > 0 {
		// 导出指定的分组
		for _, groupId := range in.GroupIds {
			group, err := l.svcCtx.AlertRuleGroupsModel.FindOne(l.ctx, groupId)
			if err != nil {
				l.Logger.Errorf("查询分组失败: %d, error: %v", groupId, err)
				continue
			}
			// 确保分组属于该文件
			if group.FileId != in.FileId {
				l.Logger.Errorf("分组 %d 不属于文件 %d", groupId, in.FileId)
				continue
			}
			groups = append(groups, group)
		}
	} else {
		// 导出该文件下的所有分组
		allGroups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "sort_order", true, "`file_id` = ?", in.FileId)
		if err != nil {
			return nil, errorx.Msg("查询分组列表失败")
		}
		groups = allGroups
	}

	if len(groups) == 0 {
		return nil, errorx.Msg("没有可导出的分组")
	}

	// 解析文件的 labels
	var fileLabels map[string]string
	if file.Labels.Valid && file.Labels.String != "" {
		if err := json.Unmarshal([]byte(file.Labels.String), &fileLabels); err != nil {
			fileLabels = make(map[string]string)
		}
	} else {
		fileLabels = map[string]string{
			"prometheus": "k8s",
			"role":       "alert-rules",
		}
	}

	// 构建 PrometheusRule
	prometheusRule := &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      file.FileCode,
			Namespace: file.Namespace,
			Labels:    fileLabels,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{},
		},
	}

	// 遍历分组构建规则
	for _, group := range groups {
		// 跳过禁用的分组
		if group.IsEnabled != 1 {
			continue
		}

		// 查询该分组下的所有规则
		rules, err := l.svcCtx.AlertRulesModel.SearchNoPage(l.ctx, "sort_order", true, "`group_id` = ?", group.Id)
		if err != nil {
			l.Logger.Errorf("查询规则列表失败: group_id=%d, error: %v", group.Id, err)
			continue
		}

		if len(rules) == 0 {
			continue
		}

		// 构建规则组
		interval := monitoringv1.Duration(group.Interval)
		ruleGroup := monitoringv1.RuleGroup{
			Name:     group.GroupName,
			Interval: &interval,
			Rules:    []monitoringv1.Rule{},
		}

		// 遍历规则
		for _, rule := range rules {
			// 跳过禁用的规则
			if rule.IsEnabled != 1 {
				continue
			}

			// 解析 labels
			var labels map[string]string
			if rule.Labels.Valid && rule.Labels.String != "" {
				if err := json.Unmarshal([]byte(rule.Labels.String), &labels); err != nil {
					labels = make(map[string]string)
				}
			} else {
				labels = make(map[string]string)
			}

			// 确保 severity 在 labels 中
			if rule.Severity != "" {
				labels["severity"] = rule.Severity
			}

			// 解析 annotations
			var annotations map[string]string
			if rule.Annotations.Valid && rule.Annotations.String != "" {
				if err := json.Unmarshal([]byte(rule.Annotations.String), &annotations); err != nil {
					annotations = make(map[string]string)
				}
			} else {
				annotations = make(map[string]string)
			}

			// 添加 summary 和 description
			if rule.Summary != "" {
				annotations["summary"] = rule.Summary
			}
			if rule.Description.Valid && rule.Description.String != "" {
				annotations["description"] = rule.Description.String
			}
			// 添加 value 模板变量
			if _, ok := annotations["value"]; !ok {
				annotations["value"] = "{{ $value }}"
			}

			// 构建 for 持续时间
			var forDuration *monitoringv1.Duration
			if rule.ForDuration != "" {
				duration := monitoringv1.Duration(rule.ForDuration)
				forDuration = &duration
			}

			ruleItem := monitoringv1.Rule{
				Alert:       rule.AlertName,
				Expr:        intstr.FromString(rule.Expr),
				For:         forDuration,
				Labels:      labels,
				Annotations: annotations,
			}

			ruleGroup.Rules = append(ruleGroup.Rules, ruleItem)
		}

		// 只有有规则的分组才添加
		if len(ruleGroup.Rules) > 0 {
			prometheusRule.Spec.Groups = append(prometheusRule.Spec.Groups, ruleGroup)
		}
	}

	if len(prometheusRule.Spec.Groups) == 0 {
		return nil, errorx.Msg("没有可导出的规则")
	}

	// 序列化为 YAML
	yamlBytes, err := yaml.Marshal(prometheusRule)
	if err != nil {
		return nil, errorx.Msg("生成 YAML 失败")
	}

	return &pb.BatchExportAlertRulesResp{
		YamlStr: string(yamlBytes),
	}, nil
}
