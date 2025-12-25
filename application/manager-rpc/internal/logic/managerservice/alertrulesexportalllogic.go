package managerservicelogic

import (
	"context"
	"encoding/json"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesExportAllLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesExportAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesExportAllLogic {
	return &AlertRulesExportAllLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRulesExportAll 导出全部告警规则为多个 YAML 文件
func (l *AlertRulesExportAllLogic) AlertRulesExportAll(in *pb.ExportAllAlertRulesReq) (*pb.ExportAllAlertRulesResp, error) {
	// 查询所有启用的告警规则文件
	files, err := l.svcCtx.AlertRuleFilesModel.SearchNoPage(l.ctx, "sort_order", true, "`is_enabled` = ? AND `is_deleted` = ?", 1, 0)
	if err != nil {
		l.Errorf("查询告警规则文件列表失败: %v", err)
		return nil, errorx.Msg("查询告警规则文件列表失败")
	}

	if len(files) == 0 {
		return nil, errorx.Msg("没有可导出的告警规则文件")
	}

	var exportedFiles []*pb.ExportedAlertRuleFile

	// 遍历每个文件进行导出
	for _, file := range files {
		// 查询该文件下的所有启用的分组
		groups, err := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "sort_order", true, "`file_id` = ? AND `is_enabled` = ? AND `is_deleted` = ?", file.Id, 1, 0)
		if err != nil {
			l.Errorf("查询文件 %d 的分组列表失败: %v", file.Id, err)
			continue
		}

		if len(groups) == 0 {
			continue
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
			// 查询该分组下的所有启用的规则
			rules, err := l.svcCtx.AlertRulesModel.SearchNoPage(l.ctx, "sort_order", true, "`group_id` = ? AND `is_enabled` = ? AND `is_deleted` = ?", group.Id, 1, 0)
			if err != nil {
				l.Errorf("查询规则列表失败: group_id=%d, error: %v", group.Id, err)
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

		// 只有有规则组的文件才导出
		if len(prometheusRule.Spec.Groups) == 0 {
			continue
		}

		// 序列化为 YAML
		yamlBytes, err := yaml.Marshal(prometheusRule)
		if err != nil {
			l.Errorf("生成文件 %s 的 YAML 失败: %v", file.FileCode, err)
			continue
		}

		exportedFiles = append(exportedFiles, &pb.ExportedAlertRuleFile{
			FileName: fmt.Sprintf("%s.yaml", file.FileCode),
			YamlStr:  string(yamlBytes),
		})
	}

	if len(exportedFiles) == 0 {
		return nil, errorx.Msg("没有可导出的告警规则")
	}

	return &pb.ExportAllAlertRulesResp{
		Files: exportedFiles,
	}, nil
}
