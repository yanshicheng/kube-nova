package managerservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"
	"sigs.k8s.io/yaml"

	"github.com/zeromicro/go-zero/core/logx"
)

type AlertRulesBatchImportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAlertRulesBatchImportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AlertRulesBatchImportLogic {
	return &AlertRulesBatchImportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// AlertRulesBatchImport 批量导入 YAML (file级别)
func (l *AlertRulesBatchImportLogic) AlertRulesBatchImport(in *pb.BatchImportAlertRulesReq) (*pb.BatchImportAlertRulesResp, error) {
	// 解析 YAML
	var prometheusRule monitoringv1.PrometheusRule
	err := yaml.Unmarshal([]byte(in.YamlStr), &prometheusRule)
	if err != nil {
		return nil, errorx.Msg("YAML格式解析失败: " + err.Error())
	}

	// 验证 kind 是否为 PrometheusRule
	if prometheusRule.Kind != "PrometheusRule" {
		return nil, errorx.Msg("YAML kind 必须是 PrometheusRule")
	}

	// 获取文件信息
	fileCode := prometheusRule.Name
	fileName := prometheusRule.Name
	namespace := prometheusRule.Namespace
	if namespace == "" {
		namespace = "monitoring"
	}

	// 序列化 labels 为 JSON
	labelsJSON := ""
	if prometheusRule.Labels != nil {
		labelsBytes, err := json.Marshal(prometheusRule.Labels)
		if err == nil {
			labelsJSON = string(labelsBytes)
		}
	}

	// 查询文件是否存在
	existFile, err := l.svcCtx.AlertRuleFilesModel.FindOneByFileCode(l.ctx, fileCode)
	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, errorx.Msg("查询告警规则文件失败")
	}

	var fileId uint64
	if existFile != nil {
		if !in.Overwrite {
			return nil, errorx.Msg("文件代码已存在，如需覆盖请设置 overwrite=true")
		}

		// 覆盖模式：删除原有的 groups 和 rules
		groups, _ := l.svcCtx.AlertRuleGroupsModel.SearchNoPage(l.ctx, "", true, "`file_id` = ?", existFile.Id)
		for _, group := range groups {
			// 删除该分组下的所有规则
			_ = l.svcCtx.AlertRulesModel.DeleteSoftByGroupId(l.ctx, group.Id)
			// 删除分组
			_ = l.svcCtx.AlertRuleGroupsModel.DeleteSoft(l.ctx, group.Id)
		}

		// 更新文件信息
		existFile.FileName = fileName
		existFile.Namespace = namespace
		existFile.Labels = sql.NullString{String: labelsJSON, Valid: labelsJSON != ""}
		existFile.UpdatedBy = in.CreatedBy
		_ = l.svcCtx.AlertRuleFilesModel.Update(l.ctx, existFile)

		fileId = existFile.Id
	} else {
		// 创建新文件
		newFile := &model.AlertRuleFiles{
			FileCode:    fileCode,
			FileName:    fileName,
			Description: fmt.Sprintf("从 PrometheusRule 导入: %s", fileName),
			Namespace:   namespace,
			Labels:      sql.NullString{String: labelsJSON, Valid: labelsJSON != ""},
			IsEnabled:   1,
			SortOrder:   0,
			CreatedBy:   in.CreatedBy,
			UpdatedBy:   in.CreatedBy,
			IsDeleted:   0,
		}

		result, err := l.svcCtx.AlertRuleFilesModel.Insert(l.ctx, newFile)
		if err != nil {
			return nil, errorx.Msg("创建告警规则文件失败")
		}

		id, err := result.LastInsertId()
		if err != nil {
			return nil, errorx.Msg("获取文件ID失败")
		}
		fileId = uint64(id)
	}

	// 遍历所有规则组并导入
	var groupCount, ruleCount int64
	for groupIdx, ruleGroup := range prometheusRule.Spec.Groups {
		// 获取分组间隔
		interval := "30s"
		if ruleGroup.Interval != nil {
			interval = string(*ruleGroup.Interval)
		}

		// 创建分组
		groupCode := ruleGroup.Name
		groupName := ruleGroup.Name

		newGroup := &model.AlertRuleGroups{
			FileId:      fileId,
			GroupCode:   groupCode,
			GroupName:   groupName,
			Description: fmt.Sprintf("规则组 %d", groupIdx+1),
			Interval:    interval,
			IsEnabled:   1,
			SortOrder:   int64(groupIdx),
			CreatedBy:   in.CreatedBy,
			UpdatedBy:   in.CreatedBy,
			IsDeleted:   0,
		}

		groupResult, err := l.svcCtx.AlertRuleGroupsModel.Insert(l.ctx, newGroup)
		if err != nil {
			l.Logger.Errorf("创建分组失败: %s, error: %v", groupCode, err)
			continue
		}

		groupId, err := groupResult.LastInsertId()
		if err != nil {
			l.Logger.Errorf("获取分组ID失败: %s, error: %v", groupCode, err)
			continue
		}
		groupCount++

		// 遍历规则并导入
		for ruleIdx, rule := range ruleGroup.Rules {
			// 跳过 record 规则，只处理 alert 规则
			if rule.Alert == "" {
				continue
			}

			// 提取规则名称
			alertName := rule.Alert
			ruleNameCn := alertName

			// 提取 severity
			severity := "warning"
			if rule.Labels != nil {
				if sev, ok := rule.Labels["severity"]; ok {
					severity = sev
				}
			}

			// 提取 summary 和 description
			summary := ""
			description := ""
			if rule.Annotations != nil {
				if sum, ok := rule.Annotations["summary"]; ok {
					summary = sum
				}
				if desc, ok := rule.Annotations["description"]; ok {
					description = desc
				}
			}

			// 序列化 labels 为 JSON
			ruleLabelsJSON := ""
			if rule.Labels != nil {
				labelsBytes, err := json.Marshal(rule.Labels)
				if err == nil {
					ruleLabelsJSON = string(labelsBytes)
				}
			}

			// 序列化 annotations 为 JSON
			annotationsJSON := ""
			if rule.Annotations != nil {
				annotationsBytes, err := json.Marshal(rule.Annotations)
				if err == nil {
					annotationsJSON = string(annotationsBytes)
				}
			}

			// 提取 for 持续时间
			forDuration := "5m"
			if rule.For != nil {
				forDuration = string(*rule.For)
			}

			// 提取表达式
			expr := rule.Expr.StrVal
			if expr == "" && rule.Expr.IntVal != 0 {
				expr = fmt.Sprintf("%d", rule.Expr.IntVal)
			}

			// 创建告警规则
			alertRule := &model.AlertRules{
				GroupId:     uint64(groupId),
				AlertName:   alertName,
				RuleNameCn:  ruleNameCn,
				Expr:        expr,
				ForDuration: forDuration,
				Severity:    severity,
				Summary:     summary,
				Description: sql.NullString{String: description, Valid: description != ""},
				Labels:      sql.NullString{String: ruleLabelsJSON, Valid: ruleLabelsJSON != ""},
				Annotations: sql.NullString{String: annotationsJSON, Valid: annotationsJSON != ""},
				IsEnabled:   1,
				SortOrder:   int64(ruleIdx),
				CreatedBy:   in.CreatedBy,
				UpdatedBy:   in.CreatedBy,
				IsDeleted:   0,
			}

			_, err = l.svcCtx.AlertRulesModel.Insert(l.ctx, alertRule)
			if err != nil {
				l.Logger.Errorf("导入告警规则失败: %s, error: %v", alertName, err)
				continue
			}
			ruleCount++
		}
	}

	return &pb.BatchImportAlertRulesResp{
		FileId:     fileId,
		GroupCount: groupCount,
		RuleCount:  ruleCount,
	}, nil
}
