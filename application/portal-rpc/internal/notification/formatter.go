package notification

import (
	"fmt"
	"html"
	"strings"
	"time"
)

// MessageFormatter 消息格式化器
// 负责将告警信息格式化为各个渠道所需的消息格式
type MessageFormatter struct {
	// PortalName 平台名称，显示在消息标题中
	PortalName string
	// PortalUrl 平台 URL，用于生成操作链接
	PortalUrl string
}

// NewMessageFormatter 创建消息格式化器
func NewMessageFormatter(portalName, portalUrl string) *MessageFormatter {
	return &MessageFormatter{
		PortalName: portalName,
		PortalUrl:  portalUrl,
	}
}

// AlertSummary 告警统计摘要
// 对告警列表按状态进行分类统计
type AlertSummary struct {
	// FiringCount 触发中的告警数量
	FiringCount int
	// ResolvedCount 已恢复的告警数量
	ResolvedCount int
	// FiringAlerts 触发中的告警列表
	FiringAlerts []*AlertInstance
	// ResolvedAlerts 已恢复的告警列表
	ResolvedAlerts []*AlertInstance
}

// AnalyzeAlerts 分析告警列表
// 将告警按状态分为触发中和已恢复两类
func (f *MessageFormatter) AnalyzeAlerts(alerts []*AlertInstance) *AlertSummary {
	summary := &AlertSummary{
		FiringAlerts:   make([]*AlertInstance, 0),
		ResolvedAlerts: make([]*AlertInstance, 0),
	}

	for _, alert := range alerts {
		if alert.Status == string(AlertStatusFiring) {
			summary.FiringCount++
			summary.FiringAlerts = append(summary.FiringAlerts, alert)
		} else {
			summary.ResolvedCount++
			summary.ResolvedAlerts = append(summary.ResolvedAlerts, alert)
		}
	}

	return summary
}

// GetSeverityLabel 获取告警级别对应的标记文本
// 返回带括号的级别文本，用于消息标题中标识级别
func (f *MessageFormatter) GetSeverityLabel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "[严重]"
	case "warning":
		return "[警告]"
	case "info":
		return "[信息]"
	case "notification":
		return "[通知]"
	default:
		return ""
	}
}

// GetSeverityColor 获取告警级别对应的颜色（带 # 前缀）
// 用于 HTML 邮件等支持颜色的渠道
func (f *MessageFormatter) GetSeverityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "info":
		return "#9e9e9e" // 灰色
	case "warning":
		return "#ffc107" // 黄色
	case "critical":
		return "#dc3545" // 红色
	default:
		return "#28a745" // 绿色（已恢复）
	}
}

// GetSeverityColorForFeiShu 获取飞书支持的告警级别颜色
// 飞书只支持预定义的颜色值，不支持自定义十六进制颜色
func (f *MessageFormatter) GetSeverityColorForFeiShu(severity string) string {
	switch strings.ToLower(severity) {
	case "critical":
		return "red"
	case "warning":
		return "orange"
	case "info":
		return "blue"
	default:
		return "green"
	}
}

// GetSeverityColorHex 获取告警级别对应的颜色（不带 # 前缀）
// 某些场景下需要不带 # 的颜色值
func (f *MessageFormatter) GetSeverityColorHex(severity string) string {
	switch strings.ToLower(severity) {
	case "info":
		return "9e9e9e"
	case "warning":
		return "ffc107"
	case "critical":
		return "dc3545"
	default:
		return "28a745"
	}
}

// FormatDuration 格式化持续时间为人类可读的中文格式
func (f *MessageFormatter) FormatDuration(seconds uint) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	} else if seconds < 3600 {
		return fmt.Sprintf("%d分钟", seconds/60)
	} else if seconds < 86400 {
		hours := seconds / 3600
		minutes := (seconds % 3600) / 60
		if minutes > 0 {
			return fmt.Sprintf("%d小时%d分钟", hours, minutes)
		}
		return fmt.Sprintf("%d小时", hours)
	} else {
		days := seconds / 86400
		hours := (seconds % 86400) / 3600
		if hours > 0 {
			return fmt.Sprintf("%d天%d小时", days, hours)
		}
		return fmt.Sprintf("%d天", days)
	}
}

// GetAlertDescription 获取告警描述
// 按优先级从 annotations 中提取描述信息
func (f *MessageFormatter) GetAlertDescription(alert *AlertInstance) string {
	// 优先使用 summary
	if summary, ok := alert.Annotations["summary"]; ok && summary != "" {
		return summary
	}
	// 其次使用 description
	if desc, ok := alert.Annotations["description"]; ok && desc != "" {
		return desc
	}
	// 最后使用 message
	if msg, ok := alert.Annotations["message"]; ok && msg != "" {
		return msg
	}
	return "暂无描述"
}

// GetAlertFiredTime 获取告警触发时间的格式化字符串
func (f *MessageFormatter) GetAlertFiredTime(alert *AlertInstance) string {
	return alert.StartsAt.Format("2006-01-02 15:04:05")
}

// minInt 返回两个整数中的较小值
// 注意: Go 1.21+ 内置了 min 函数，为避免冲突这里重命名为 minInt
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// buildTitle 构建消息标题
// 根据是否有级别标记来决定标题格式
func (f *MessageFormatter) buildTitle(label, suffix string) string {
	if label != "" {
		return fmt.Sprintf("%s %s %s", label, f.PortalName, suffix)
	}
	return fmt.Sprintf("%s %s", f.PortalName, suffix)
}

// FormatMarkdownForDingTalk 为钉钉格式化 Markdown 消息
// 返回标题和正文内容，企业级专业报告风格
// 注意: 钉钉@人需要在消息内容末尾添加@人标记
func (f *MessageFormatter) FormatMarkdownForDingTalk(opts *AlertOptions, alerts []*AlertInstance) (title, content string) {
	summary := f.AnalyzeAlerts(alerts)
	label := f.GetSeverityLabel(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	// 项目显示名称，集群级告警显示为"集群级"
	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	title = f.buildTitle(label, "告警通知")

	var sb strings.Builder

	// 主标题 - 使用更清晰的格式
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))
	sb.WriteString("---\n\n")

	// 基本信息 - 使用表格式布局
	sb.WriteString("### 📊 告警概况\n\n")
	sb.WriteString(fmt.Sprintf("**项目**: %s\n\n", projectDisplay))
	sb.WriteString(fmt.Sprintf("**集群**: %s\n\n", opts.ClusterName))
	sb.WriteString(fmt.Sprintf("**级别**: %s\n\n", strings.ToUpper(opts.Severity)))
	sb.WriteString(fmt.Sprintf("**时间**: %s\n\n", now))

	// 统计信息
	if summary.FiringCount > 0 || summary.ResolvedCount > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString("### 📈 状态统计\n\n")
		if summary.FiringCount > 0 {
			sb.WriteString(fmt.Sprintf("🔴 触发中: **%d** 条\n\n", summary.FiringCount))
		}
		if summary.ResolvedCount > 0 {
			sb.WriteString(fmt.Sprintf("🟢 已恢复: **%d** 条\n\n", summary.ResolvedCount))
		}
	}

	// 告警详情（最多显示 3 条）
	if summary.FiringCount > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("### 🚨 告警详情 (前 %d 条)\n\n", minInt(3, summary.FiringCount)))
		displayCount := minInt(3, summary.FiringCount)

		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]

			// 提取摘要和描述
			summaryText := alert.Annotations["summary"]
			description := alert.Annotations["description"]
			value := alert.Annotations["value"]

			// 格式化实例名
			instanceText := alert.Instance
			if len(instanceText) > 50 {
				instanceText = instanceText[:47] + "..."
			}

			sb.WriteString(fmt.Sprintf("**%d. %s**\n\n", i+1, summaryText))
			sb.WriteString(fmt.Sprintf("- 实例: `%s`\n\n", instanceText))
			sb.WriteString(fmt.Sprintf("- 触发时间: %s\n\n", f.GetAlertFiredTime(alert)))
			sb.WriteString(fmt.Sprintf("- 持续时间: %s", f.FormatDuration(alert.Duration)))
			if alert.RepeatCount > 1 {
				sb.WriteString(fmt.Sprintf(" (重复 %d 次)", alert.RepeatCount))
			}
			sb.WriteString("\n\n")

			// 如果有值信息，显示阈值
			if value != "" {
				sb.WriteString(fmt.Sprintf("- 当前值: %s\n\n", value))
			}

			// 描述信息
			if description != "" && description != "暂无描述" {
				desc := description
				if len(desc) > 100 {
					desc = desc[:97] + "..."
				}
				sb.WriteString(fmt.Sprintf("- 描述: %s\n\n", desc))
			}
		}

		if summary.FiringCount > 3 {
			sb.WriteString(fmt.Sprintf("...还有 **%d** 条告警未显示\n\n", summary.FiringCount-3))
		}
	}

	// 恢复通知（最多显示 2 条）
	if summary.ResolvedCount > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString("### ✅ 已恢复\n\n")
		displayCount := minInt(2, summary.ResolvedCount)

		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 50 {
				instanceText = instanceText[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. %s - `%s`\n\n", i+1, alert.AlertName, instanceText))
		}
	}

	// 操作链接
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("[🔗 查看详情](%s)", f.PortalUrl))

	content = sb.String()
	return
}

// FormatNotificationForDingTalk 为钉钉格式化通知消息
func (f *MessageFormatter) FormatNotificationForDingTalk(opts *NotificationOptions) (title, content string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	label := f.GetSeverityLabel("notification")

	title = f.buildTitle(label, opts.Title)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#### %s %s 通知\n\n", f.PortalName, opts.Title))
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("> **时间**: %s\n\n", now))
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", opts.Content))
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("[前往控制台](%s)", f.PortalUrl))

	content = sb.String()
	return
}

// FormatMarkdownForWeChat 为企业微信格式化 Markdown 消息
// 注意: 企业微信的 markdown 支持有限的格式
func (f *MessageFormatter) FormatMarkdownForWeChat(opts *AlertOptions, alerts []*AlertInstance) string {
	summary := f.AnalyzeAlerts(alerts)
	label := f.GetSeverityLabel(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	var sb strings.Builder

	// 主标题
	sb.WriteString(fmt.Sprintf("# %s\n\n", f.buildTitle(label, "告警通知")))

	// 告警概况 - 使用引用块
	sb.WriteString("> **告警概况**\n")
	sb.WriteString(fmt.Sprintf("> 项目: %s\n", projectDisplay))
	sb.WriteString(fmt.Sprintf("> 集群: %s\n", opts.ClusterName))
	sb.WriteString(fmt.Sprintf("> 级别: <font color=\"warning\">%s</font>\n", strings.ToUpper(opts.Severity)))
	sb.WriteString(fmt.Sprintf("> 时间: %s\n\n", now))

	// 状态统计
	if summary.FiringCount > 0 || summary.ResolvedCount > 0 {
		sb.WriteString("**状态统计**\n")
		if summary.FiringCount > 0 {
			sb.WriteString(fmt.Sprintf("触发中: <font color=\"warning\">%d</font> 条\n", summary.FiringCount))
		}
		if summary.ResolvedCount > 0 {
			sb.WriteString(fmt.Sprintf("已恢复: <font color=\"info\">%d</font> 条\n", summary.ResolvedCount))
		}
		sb.WriteString("\n")
	}

	// 告警详情（最多显示 2 条，企业微信消息不宜过长）
	if summary.FiringCount > 0 {
		displayCount := minInt(2, len(summary.FiringAlerts))
		sb.WriteString(fmt.Sprintf("**告警详情** (显示前 %d 条)\n\n", displayCount))

		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]

			// 提取信息
			summaryText := alert.Annotations["summary"]
			description := alert.Annotations["description"]
			value := alert.Annotations["value"]

			// 格式化实例名
			instanceText := alert.Instance
			if len(instanceText) > 50 {
				instanceText = instanceText[:47] + "..."
			}

			sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, summaryText))
			sb.WriteString(fmt.Sprintf("> 实例: `%s`\n", instanceText))
			sb.WriteString(fmt.Sprintf("> 触发时间: %s\n", f.GetAlertFiredTime(alert)))
			sb.WriteString(fmt.Sprintf("> 持续时间: %s", f.FormatDuration(alert.Duration)))
			if alert.RepeatCount > 1 {
				sb.WriteString(fmt.Sprintf(" (重复 %d 次)", alert.RepeatCount))
			}
			sb.WriteString("\n")

			// 当前值
			if value != "" {
				sb.WriteString(fmt.Sprintf("> 当前值: %s\n", value))
			}

			// 描述
			if description != "" && description != "暂无描述" {
				desc := description
				if len(desc) > 100 {
					desc = desc[:97] + "..."
				}
				sb.WriteString(fmt.Sprintf("> 描述: %s\n", desc))
			}
			sb.WriteString("\n")
		}

		if summary.FiringCount > 2 {
			sb.WriteString(fmt.Sprintf("...还有 %d 条告警未显示\n\n", summary.FiringCount-2))
		}
	}

	// 恢复通知
	if summary.ResolvedCount > 0 {
		sb.WriteString("**已恢复**\n\n")
		displayCount := minInt(2, len(summary.ResolvedAlerts))

		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 50 {
				instanceText = instanceText[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. %s - `%s`\n", i+1, alert.AlertName, instanceText))
		}
		sb.WriteString("\n")
	}

	// 操作链接
	sb.WriteString(fmt.Sprintf("[查看详情](%s)\n", f.PortalUrl))

	return sb.String()
}

// FormatNotificationForWeChat 为企业微信格式化通知消息
func (f *MessageFormatter) FormatNotificationForWeChat(opts *NotificationOptions) string {
	now := time.Now().Format("2006-01-02 15:04:05")
	label := f.GetSeverityLabel("notification")

	var sb strings.Builder
	if label != "" {
		sb.WriteString(fmt.Sprintf("### %s %s %s 通知\n\n", label, f.PortalName, opts.Title))
	} else {
		sb.WriteString(fmt.Sprintf("### %s %s 通知\n\n", f.PortalName, opts.Title))
	}

	sb.WriteString(fmt.Sprintf("> **时间**: %s\n\n", now))
	sb.WriteString(fmt.Sprintf("%s\n\n", opts.Content))
	sb.WriteString(fmt.Sprintf("[前往控制台](%s)\n", f.PortalUrl))

	return sb.String()
}

// FormatRichTextForFeiShu 为飞书格式化富文本消息
// 返回标题和结构化的内容数组
// 按级别分组显示，支持同一告警组多个级别聚合
func (f *MessageFormatter) FormatRichTextForFeiShu(opts *AlertOptions, alerts []*AlertInstance) (title string, content [][]map[string]interface{}) {
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	title = f.PortalName + " 告警通知"

	// 按级别分组告警
	alertsBySeverity := make(map[string][]*AlertInstance)
	resolvedAlerts := make([]*AlertInstance, 0)
	clusterStats := make(map[string]struct{ firing, resolved int })

	for _, alert := range alerts {
		if alert.Status == "firing" {
			severity := strings.ToUpper(alert.Severity)
			alertsBySeverity[severity] = append(alertsBySeverity[severity], alert)

			// 统计集群维度
			stats := clusterStats[alert.ClusterName]
			stats.firing++
			clusterStats[alert.ClusterName] = stats
		} else {
			resolvedAlerts = append(resolvedAlerts, alert)

			// 统计集群维度
			stats := clusterStats[alert.ClusterName]
			stats.resolved++
			clusterStats[alert.ClusterName] = stats
		}
	}

	// 计算总数
	totalFiring := 0
	for _, alerts := range alertsBySeverity {
		totalFiring += len(alerts)
	}
	totalResolved := len(resolvedAlerts)

	// 构建消息内容
	content = [][]map[string]interface{}{
		{{"tag": "text", "text": fmt.Sprintf("【%s】", title)}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": "📊 告警概况"}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": fmt.Sprintf("项目: %s", projectDisplay)}},
		{{"tag": "text", "text": fmt.Sprintf("时间: %s", now)}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": "📈 状态统计"}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": fmt.Sprintf("🔴 触发中: %d条", totalFiring)}},
		{{"tag": "text", "text": fmt.Sprintf("🟢 已恢复: %d条", totalResolved)}},
		{{"tag": "text", "text": ""}},
	}

	// 按集群统计
	if len(clusterStats) > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": "按集群统计:"},
		})
		for cluster, stats := range clusterStats {
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("• %s: %d条触发 / %d条恢复", cluster, stats.firing, stats.resolved)},
			})
		}
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
	}

	// 按级别统计
	if len(alertsBySeverity) > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": "按级别统计:"},
		})
		// 按优先级排序：CRITICAL > WARNING > INFO
		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, sev := range severityOrder {
			if count := len(alertsBySeverity[sev]); count > 0 {
				content = append(content, []map[string]interface{}{
					{"tag": "text", "text": fmt.Sprintf("• %s: %d条", sev, count)},
				})
			}
		}
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
	}

	// 告警详情 - 按级别分组显示
	content = append(content, []map[string]interface{}{
		{"tag": "text", "text": ""},
	})
	content = append(content, []map[string]interface{}{
		{"tag": "text", "text": "🚨 告警详情"},
	})
	content = append(content, []map[string]interface{}{
		{"tag": "text", "text": ""},
	})
	content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})

	// 按优先级显示各级别告警
	severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
	for _, severity := range severityOrder {
		alerts := alertsBySeverity[severity]
		if len(alerts) == 0 {
			continue
		}

		// 级别标题
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("【%s 级别】(%d条)", severity, len(alerts))},
		})
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})

		// 显示该级别的所有告警
		for i, alert := range alerts {
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}

			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("%d. %s", i+1, summaryText)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 集群: %s", alert.ClusterName)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 实例: %s", alert.Instance)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 触发时间: %s", f.GetAlertFiredTime(alert))},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 持续时间: %s", f.FormatDuration(alert.Duration))},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 重复次数: %d次", alert.RepeatCount)},
			})

			// 当前值
			if value := alert.Annotations["value"]; value != "" {
				content = append(content, []map[string]interface{}{
					{"tag": "text", "text": fmt.Sprintf("   • 当前值: %s", value)},
				})
			}

			// 描述
			desc := f.GetAlertDescription(alert)
			if desc != "暂无描述" && desc != "" {
				content = append(content, []map[string]interface{}{
					{"tag": "text", "text": fmt.Sprintf("   • 描述: %s", desc)},
				})
			}

			content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
		}
	}

	// 已恢复告警
	if len(resolvedAlerts) > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": ""},
		})
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("✅ 已恢复告警 (%d条)", len(resolvedAlerts))},
		})
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": ""},
		})
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})

		for i, alert := range resolvedAlerts {
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}

			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("%d. %s", i+1, summaryText)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 集群: %s", alert.ClusterName)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 实例: %s", alert.Instance)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 恢复时间: %s", resolvedTime)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   • 持续时长: %s", f.FormatDuration(alert.Duration))},
			})
			content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
		}
	}

	// 底部操作链接
	content = append(content, []map[string]interface{}{
		{"tag": "text", "text": ""},
	})
	content = append(content, []map[string]interface{}{
		{"tag": "a", "text": "🔗 查看详情", "href": f.PortalUrl},
	})

	return
}

// FormatNotificationForFeiShu 为飞书格式化通知消息
// 企业级专业报告风格
func (f *MessageFormatter) FormatNotificationForFeiShu(opts *NotificationOptions) (title string, content [][]map[string]interface{}) {
	now := time.Now().Format("2006-01-02 15:04:05")
	label := f.GetSeverityLabel("notification")

	title = f.buildTitle(label, opts.Title+" 通知")

	content = [][]map[string]interface{}{
		{{"tag": "text", "text": ""}},
		// 标题
		{{"tag": "text", "text": fmt.Sprintf("%s %s 通知", f.PortalName, opts.Title)}},
		{{"tag": "text", "text": ""}},
		// 时间信息
		{{"tag": "text", "text": fmt.Sprintf("时间: %s", now)}},
		{{"tag": "text", "text": ""}},
		// 分隔线
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": ""}},
		// 通知内容
		{{"tag": "text", "text": "【通知内容】"}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": opts.Content}},
		{{"tag": "text", "text": ""}},
		// 操作链接
		{{"tag": "a", "text": "前往控制台", "href": f.PortalUrl}},
	}

	return
}

// FormatHTMLForEmail 为邮件格式化 HTML 消息
// 返回邮件主题和 HTML 正文
// 注意: 所有用户输入都经过 HTML 转义以防止 XSS 攻击
// 优化: 添加最大高度限制和滚动查看，提供更全面的告警信息
func (f *MessageFormatter) FormatHTMLForEmail(opts *AlertOptions, alerts []*AlertInstance) (subject, body string) {
	summary := f.AnalyzeAlerts(alerts)
	label := f.GetSeverityLabel(opts.Severity)
	color := f.GetSeverityColor(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	// 构建邮件主题
	subject = fmt.Sprintf("[%s] %s - %s 告警 (%d告警/%d恢复)",
		strings.ToUpper(opts.Severity),
		html.EscapeString(f.PortalName),
		html.EscapeString(opts.ClusterName),
		summary.FiringCount,
		summary.ResolvedCount)

	// 对用户输入进行 HTML 转义，防止 XSS 攻击
	escapedPortalName := html.EscapeString(f.PortalName)
	escapedProjectDisplay := html.EscapeString(projectDisplay)
	escapedClusterName := html.EscapeString(opts.ClusterName)
	escapedSeverity := html.EscapeString(strings.ToUpper(opts.Severity))

	body = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;line-height:1.6;color:#333;margin:0;padding:20px;background:#f5f5f5}
.email-container{max-width:900px;min-height:200px;max-height:800px;margin:0 auto;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.12)}
.header{background:%s;color:#fff;padding:28px 32px;text-align:center;border-bottom:4px solid rgba(0,0,0,0.1)}
.header h1{margin:0;font-size:22px;font-weight:700;letter-spacing:-0.5px}
.header p{margin:10px 0 0;opacity:0.95;font-size:14px;font-weight:500}
.content{padding:28px 32px;max-height:600px;overflow-y:auto}
.content::-webkit-scrollbar{width:8px}
.content::-webkit-scrollbar-track{background:#f1f1f1;border-radius:4px}
.content::-webkit-scrollbar-thumb{background:#888;border-radius:4px}
.content::-webkit-scrollbar-thumb:hover{background:#555}
.info-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-bottom:24px;padding:20px;background:linear-gradient(135deg,#f8f9fa 0%%,#e9ecef 100%%);border-radius:10px;border:1px solid #dee2e6}
.info-item{padding:12px;background:#fff;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,0.05)}
.info-label{font-size:11px;color:#6c757d;margin-bottom:6px;text-transform:uppercase;letter-spacing:0.5px;font-weight:600}
.info-value{font-size:15px;font-weight:700;color:#212529}
.stats{display:grid;grid-template-columns:repeat(2,1fr);gap:16px;margin-bottom:28px}
.stat{padding:20px;border-radius:10px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08);transition:transform 0.2s}
.stat.firing{background:linear-gradient(135deg,#fff5f5 0%%,#ffe5e5 100%%);border:2px solid #ff4d4f}
.stat.resolved{background:linear-gradient(135deg,#f6ffed 0%%,#d9f7be 100%%);border:2px solid #52c41a}
.stat-num{font-size:36px;font-weight:800;line-height:1.2;margin-bottom:8px}
.stat-num.firing{color:#cf1322}
.stat-num.resolved{color:#389e0d}
.stat-label{font-size:13px;color:#595959;font-weight:600;text-transform:uppercase;letter-spacing:0.5px}
.section{margin-bottom:24px}
.section-title{font-size:16px;font-weight:700;color:#262626;margin-bottom:16px;padding:12px 16px;background:#fafafa;border-left:4px solid %s;border-radius:4px}
.alert-list{display:flex;flex-direction:column;gap:16px}
.alert-item{padding:20px;background:#fafafa;border-radius:10px;border-left:5px solid %s;box-shadow:0 2px 6px rgba(0,0,0,0.06);transition:box-shadow 0.2s}
.alert-item:hover{box-shadow:0 4px 12px rgba(0,0,0,0.1)}
.alert-item.resolved{border-left-color:#52c41a;background:linear-gradient(135deg,#f9fff6 0%%,#f0f9ff 100%%)}
.alert-header{font-weight:700;color:#262626;margin-bottom:12px;font-size:16px;display:flex;align-items:center;gap:8px}
.alert-header::before{content:'🔴';font-size:14px}
.alert-item.resolved .alert-header::before{content:'✅'}
.alert-meta{font-size:13px;color:#595959;line-height:2;background:#fff;padding:12px;border-radius:6px;margin-bottom:12px}
.alert-meta-row{margin-bottom:6px;display:flex;align-items:baseline}
.alert-meta-label{display:inline-block;min-width:100px;color:#8c8c8c;font-weight:600;font-size:12px}
.alert-meta-value{flex:1;color:#262626;font-weight:500}
.alert-meta code{background:#f0f0f0;padding:3px 8px;border-radius:4px;font-family:'SF Mono',Monaco,Consolas,monospace;font-size:12px;color:#d73a49;border:1px solid #e1e4e8}
.alert-desc{margin-top:12px;padding:12px 16px;background:#fff;border-radius:6px;font-size:14px;color:#595959;line-height:1.8;border-left:3px solid #1890ff}
.alert-labels{margin-top:12px;display:flex;flex-wrap:wrap;gap:8px}
.label-tag{display:inline-block;padding:4px 10px;background:#e6f7ff;color:#0050b3;border-radius:4px;font-size:11px;font-weight:600;border:1px solid #91d5ff}
.more-info{text-align:center;padding:16px;color:#8c8c8c;font-size:14px;font-weight:600;background:#fafafa;border-radius:8px;margin-top:12px}
.btn{display:inline-block;background:%s;color:#fff;padding:14px 40px;text-decoration:none;border-radius:8px;font-weight:700;font-size:15px;margin-top:20px;box-shadow:0 4px 12px rgba(0,0,0,0.15);transition:all 0.3s}
.btn:hover{transform:translateY(-2px);box-shadow:0 6px 16px rgba(0,0,0,0.2)}
.footer{padding:20px 32px;background:#fafafa;text-align:center;font-size:12px;color:#8c8c8c;border-top:1px solid #e8e8e8}
.footer-warning{color:#ff4d4f;font-weight:700;margin-top:10px;font-size:13px}
@media (max-width:768px){
.email-container{margin:0;border-radius:0;max-height:none}
.content{padding:20px;max-height:none}
.info-grid{grid-template-columns:1fr;gap:12px}
.stats{grid-template-columns:1fr}
}
</style>
</head>
<body>
<div class="email-container">
<div class="header">
<h1>%s %s 告警通知</h1>
<p>%s</p>
</div>
<div class="content">
<div class="info-grid">
<div class="info-item"><div class="info-label">项目</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">集群</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">级别</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">通知时间</div><div class="info-value">%s</div></div>
</div>
<div class="stats">
<div class="stat firing"><div class="stat-num firing">%d</div><div class="stat-label">触发告警</div></div>
<div class="stat resolved"><div class="stat-num resolved">%d</div><div class="stat-label">已恢复</div></div>
</div>`,
		color, color, color, color,
		label, escapedPortalName, now,
		escapedProjectDisplay, escapedClusterName, escapedSeverity, now,
		summary.FiringCount, summary.ResolvedCount)

	// 告警详情（显示所有告警，邮件没有大小限制）
	if summary.FiringCount > 0 {
		body += `<div class="section"><div class="section-title">🚨 告警详情</div><div class="alert-list">`

		for i, alert := range summary.FiringAlerts {
			firedTime := f.GetAlertFiredTime(alert)

			// 转义所有用户输入
			escapedSummary := html.EscapeString(alert.Annotations["summary"])
			if escapedSummary == "" {
				escapedSummary = html.EscapeString(alert.AlertName)
			}
			escapedInstance := html.EscapeString(alert.Instance)
			escapedValue := html.EscapeString(alert.Annotations["value"])
			escapedDescription := html.EscapeString(alert.Annotations["description"])
			escapedAlertDesc := html.EscapeString(f.GetAlertDescription(alert))

			body += fmt.Sprintf(`<div class="alert-item">
<div class="alert-header">%d. %s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">实例标识</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">触发时间</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">持续时间</span><span class="alert-meta-value">%s</span></div>`,
				i+1, escapedSummary, escapedInstance, firedTime, f.FormatDuration(alert.Duration))

			if escapedValue != "" {
				body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">当前值</span><span class="alert-meta-value">%s</span></div>`, escapedValue)
			}

			body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">重复次数</span><span class="alert-meta-value">%d 次</span></div>`, alert.RepeatCount)

			if escapedDescription != "" && escapedDescription != "暂无描述" {
				body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">告警详情</span><span class="alert-meta-value">%s</span></div>`, escapedDescription)
			}

			body += `</div>`

			if escapedAlertDesc != "暂无描述" && escapedAlertDesc != "" {
				body += fmt.Sprintf(`<div class="alert-desc">📝 %s</div>`, escapedAlertDesc)
			}

			// 显示关键标签
			if len(alert.Labels) > 0 {
				body += `<div class="alert-labels">`
				labelCount := 0
				for key, value := range alert.Labels {
					if labelCount >= 8 {
						break
					}
					// 只显示重要标签
					if key == "namespace" || key == "pod" || key == "node" || key == "job" || key == "service" || key == "deployment" {
						escapedKey := html.EscapeString(key)
						escapedVal := html.EscapeString(value)
						body += fmt.Sprintf(`<span class="label-tag">%s: %s</span>`, escapedKey, escapedVal)
						labelCount++
					}
				}
				body += `</div>`
			}

			body += `</div>`
		}

		body += `</div></div>`
	}

	// 恢复通知（显示所有恢复）
	if summary.ResolvedCount > 0 {
		body += `<div class="section"><div class="section-title">✅ 已恢复告警</div><div class="alert-list">`

		for i, alert := range summary.ResolvedAlerts {
			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			// 转义用户输入
			escapedSummary := html.EscapeString(alert.Annotations["summary"])
			if escapedSummary == "" {
				escapedSummary = html.EscapeString(alert.AlertName)
			}
			escapedAlertName := html.EscapeString(alert.AlertName)
			escapedInstance := html.EscapeString(alert.Instance)

			body += fmt.Sprintf(`<div class="alert-item resolved">
<div class="alert-header">%d. %s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">告警规则</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">实例标识</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">恢复时间</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">持续时长</span><span class="alert-meta-value">%s</span></div>
</div>
</div>`, i+1, escapedSummary, escapedAlertName, escapedInstance, resolvedTime, f.FormatDuration(alert.Duration))
		}

		body += `</div></div>`
	}

	// 操作按钮和页脚
	body += fmt.Sprintf(`<div style="text-align:center;margin-top:32px"><a href="%s" class="btn">🔗 立即处理告警</a></div>
</div>
<div class="footer">
<div>此邮件由 %s 自动发送</div>
<div class="footer-warning">⚠️ 请及时处理告警，系统自动发出请勿回复</div>
</div>
</div>
</body>
</html>`, f.PortalUrl, escapedPortalName)

	return
}

// FormatNotificationForEmail 为邮件格式化通知消息
func (f *MessageFormatter) FormatNotificationForEmail(opts *NotificationOptions) (subject, body string) {
	now := time.Now().Format("2006-01-02 15:04:05")

	// 转义用户输入
	escapedPortalName := html.EscapeString(f.PortalName)
	escapedTitle := html.EscapeString(opts.Title)
	escapedContent := html.EscapeString(opts.Content)

	subject = fmt.Sprintf("[通知] %s - %s", escapedPortalName, escapedTitle)

	body = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body {
			font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
			line-height: 1.6;
			color: #333;
			margin: 0;
			padding: 20px;
			background: #f5f5f5;
		}
		.card {
			max-width: 900px;
			margin: 0 auto;
			background: #fff;
			border-radius: 12px;
			overflow: hidden;
			box-shadow: 0 4px 12px rgba(0,0,0,0.1);
		}
		.header {
			background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
			color: #fff;
			padding: 32px;
			text-align: center;
		}
		.header h1 {
			margin: 0;
			font-size: 24px;
			font-weight: 600;
		}
		.header p {
			margin: 8px 0 0;
			opacity: 0.9;
			font-size: 14px;
		}
		.content {
			padding: 32px;
		}
		.message {
			padding: 24px;
			background: #f8f9fa;
			border-radius: 8px;
			margin-bottom: 24px;
			white-space: pre-wrap;
			font-size: 15px;
			line-height: 1.8;
			min-height: 200px;
		}
		.btn {
			display: inline-block;
			background: #667eea;
			color: #fff;
			padding: 12px 32px;
			text-decoration: none;
			border-radius: 6px;
			font-weight: 600;
			transition: background 0.3s;
		}
		.btn:hover {
			background: #5568d3;
		}
		.footer {
			padding: 20px 32px;
			background: #fafafa;
			text-align: center;
			font-size: 12px;
			color: #999;
		}
		.footer-warning {
			color: #ff4d4f;
			font-weight: 600;
			margin-top: 8px;
		}
		@media (max-width: 768px) {
			.card {
				margin: 0;
				border-radius: 0;
			}
			.content {
				padding: 20px;
			}
		}
	</style>
</head>
<body>
	<div class="card">
		<div class="header">
			<h1>%s</h1>
			<p>%s</p>
		</div>
		<div class="content">
			<div class="message">%s</div>
			<div style="text-align:center">
				<a href="%s" class="btn">前往控制台</a>
			</div>
		</div>
		<div class="footer">
			<div>此邮件由 %s 自动发送</div>
			<div class="footer-warning">系统自动发出，请勿回复</div>
		</div>
	</div>
</body>
</html>`, escapedTitle, now, escapedContent, f.PortalUrl, escapedPortalName)

	return
}

// FormatAggregatedAlertForDingTalk 为钉钉格式化聚合后的多级别告警消息
// 支持同一项目多个级别的告警统一展示
func (f *MessageFormatter) FormatAggregatedAlertForDingTalk(group *AggregatedAlertGroup) (title, content string) {
	now := time.Now().Format("2006-01-02 15:04:05")

	// 项目显示名称
	projectDisplay := group.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	// 标题：使用最高级别的标签
	highestSeverity := f.getHighestSeverity(group.AlertsBySeverity)
	label := f.GetSeverityLabel(highestSeverity)
	title = f.buildTitle(label, "告警通知")

	var sb strings.Builder

	// 主标题
	sb.WriteString(fmt.Sprintf("## %s\n\n", title))

	// 告警概况
	sb.WriteString("### 📊 告警概况\n\n")
	sb.WriteString(fmt.Sprintf("**项目**: %s\n\n", projectDisplay))
	sb.WriteString(fmt.Sprintf("**时间**: %s\n\n", now))

	// 状态统计
	sb.WriteString("### 📈 状态统计\n\n")
	sb.WriteString(fmt.Sprintf("🔴 触发中: **%d** 条\n\n", group.TotalFiring))
	if group.TotalResolved > 0 {
		sb.WriteString(fmt.Sprintf("🟢 已恢复: **%d** 条\n\n", group.TotalResolved))
	}

	// 按集群统计
	if len(group.ClusterStats) > 0 {
		for clusterName, stat := range group.ClusterStats {
			sb.WriteString(fmt.Sprintf("**%s集群**: %d条触发 / %d条恢复\n\n", clusterName, stat.FiringCount, stat.ResolvedCount))
		}
	}

	// 告警详情 - 按级别分组显示
	if group.TotalFiring > 0 {
		sb.WriteString("### 🚨 告警详情\n\n")

		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, severity := range severityOrder {
			alerts, ok := group.AlertsBySeverity[severity]
			if !ok || len(alerts) == 0 {
				continue
			}

			// 级别标题
			sb.WriteString(fmt.Sprintf("**级别: %s**\n\n", severity))

			// 显示该级别的所有告警
			for i, alert := range alerts {
				summaryText := alert.Annotations["summary"]
				if summaryText == "" {
					summaryText = alert.AlertName
				}
				description := alert.Annotations["description"]
				value := alert.Annotations["value"]

				instanceText := alert.Instance
				if len(instanceText) > 50 {
					instanceText = instanceText[:47] + "..."
				}

				sb.WriteString(fmt.Sprintf("**%d. %s**\n\n", i+1, summaryText))
				sb.WriteString(fmt.Sprintf("- 告警集群: %s\n\n", alert.ClusterName))
				sb.WriteString(fmt.Sprintf("- 告警实例: `%s`\n\n", instanceText))
				sb.WriteString(fmt.Sprintf("- 触发时间: %s\n\n", f.GetAlertFiredTime(alert)))
				sb.WriteString(fmt.Sprintf("- 持续时间: %s", f.FormatDuration(alert.Duration)))
				if alert.RepeatCount > 1 {
					sb.WriteString(fmt.Sprintf(" (重复 %d 次)", alert.RepeatCount))
				}
				sb.WriteString("\n\n")

				if value != "" {
					sb.WriteString(fmt.Sprintf("- 当前值: %s\n\n", value))
				}

				if description != "" && description != "暂无描述" {
					desc := description
					if len(desc) > 100 {
						desc = desc[:97] + "..."
					}
					sb.WriteString(fmt.Sprintf("- 描述: %s\n\n", desc))
				}
			}
		}
	}

	// 已恢复告警
	if group.TotalResolved > 0 {
		sb.WriteString("### ✅ 已恢复告警\n\n")
		displayCount := minInt(3, len(group.ResolvedAlerts))

		for i := 0; i < displayCount; i++ {
			alert := group.ResolvedAlerts[i]
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}
			instanceText := alert.Instance
			if len(instanceText) > 50 {
				instanceText = instanceText[:47] + "..."
			}

			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			sb.WriteString(fmt.Sprintf("%d. %s - `%s`\n\n", i+1, summaryText, instanceText))
			sb.WriteString(fmt.Sprintf("   • 集群: %s\n\n", alert.ClusterName))
			sb.WriteString(fmt.Sprintf("   • 恢复时间: %s\n\n", resolvedTime))
			sb.WriteString(fmt.Sprintf("   • 持续时长: %s\n\n", f.FormatDuration(alert.Duration)))
		}

		if len(group.ResolvedAlerts) > 3 {
			sb.WriteString(fmt.Sprintf("...还有 **%d** 条已恢复告警未显示\n\n", len(group.ResolvedAlerts)-3))
		}
	}

	// 操作链接
	sb.WriteString(fmt.Sprintf("[🔗 查看详情](%s)", f.PortalUrl))

	content = sb.String()
	return
}

// getHighestSeverity 获取最高级别
func (f *MessageFormatter) getHighestSeverity(alertsBySeverity map[string][]*AlertInstance) string {
	if len(alertsBySeverity["CRITICAL"]) > 0 || len(alertsBySeverity["critical"]) > 0 {
		return "critical"
	}
	if len(alertsBySeverity["WARNING"]) > 0 || len(alertsBySeverity["warning"]) > 0 {
		return "warning"
	}
	if len(alertsBySeverity["INFO"]) > 0 || len(alertsBySeverity["info"]) > 0 {
		return "info"
	}
	return "info"
}

// FormatAggregatedAlertForWeChat 为企业微信格式化聚合后的多级别告警消息
// 考虑企业微信的消息长度限制（2048字符），严格控制显示数量
func (f *MessageFormatter) FormatAggregatedAlertForWeChat(group *AggregatedAlertGroup) string {
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := group.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	highestSeverity := f.getHighestSeverity(group.AlertsBySeverity)
	label := f.GetSeverityLabel(highestSeverity)

	var sb strings.Builder

	// 主标题
	sb.WriteString(fmt.Sprintf("# %s\n\n", f.buildTitle(label, "告警通知")))

	// 告警概况
	sb.WriteString("> **告警概况**\n")
	sb.WriteString(fmt.Sprintf("> 项目: %s\n", projectDisplay))
	sb.WriteString(fmt.Sprintf("> 级别: <font color=\"warning\">%s</font>\n", strings.ToUpper(highestSeverity)))
	sb.WriteString(fmt.Sprintf("> 时间: %s\n\n", now))

	// 状态统计
	sb.WriteString("**状态统计**\n")
	sb.WriteString(fmt.Sprintf("触发中: <font color=\"warning\">%d</font> 条\n", group.TotalFiring))
	if group.TotalResolved > 0 {
		sb.WriteString(fmt.Sprintf("已恢复: <font color=\"info\">%d</font> 条\n", group.TotalResolved))
	}

	// 按集群统计
	if len(group.ClusterStats) > 0 {
		count := 0
		for clusterName, stat := range group.ClusterStats {
			if count >= 3 {
				sb.WriteString(fmt.Sprintf("...还有 %d 个集群\n", len(group.ClusterStats)-3))
				break
			}
			sb.WriteString(fmt.Sprintf("**%s集群**: %d触发/%d恢复\n", clusterName, stat.FiringCount, stat.ResolvedCount))
			count++
		}
	}

	// 告警详情 - 按级别分组显示
	if group.TotalFiring > 0 {
		sb.WriteString("\n**告警详情**\n\n")

		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, severity := range severityOrder {
			alerts, ok := group.AlertsBySeverity[severity]
			if !ok || len(alerts) == 0 {
				continue
			}

			sb.WriteString(fmt.Sprintf("**级别: %s**\n", severity))

			displayCount := minInt(2, len(alerts))
			for i := 0; i < displayCount; i++ {
				alert := alerts[i]

				summaryText := alert.Annotations["summary"]
				if summaryText == "" {
					summaryText = alert.AlertName
				}

				instanceText := alert.Instance
				if len(instanceText) > 40 {
					instanceText = instanceText[:37] + "..."
				}

				sb.WriteString(fmt.Sprintf("**%d. %s**\n", i+1, summaryText))
				sb.WriteString(fmt.Sprintf("> 告警集群: %s\n", alert.ClusterName))
				sb.WriteString(fmt.Sprintf("> 告警实例: `%s`\n", instanceText))
				sb.WriteString(fmt.Sprintf("> 触发时间: %s\n", f.GetAlertFiredTime(alert)))
				sb.WriteString(fmt.Sprintf("> 持续时间: %s\n", f.FormatDuration(alert.Duration)))

				if description := alert.Annotations["description"]; description != "" && description != "暂无描述" {
					desc := description
					if len(desc) > 80 {
						desc = desc[:77] + "..."
					}
					sb.WriteString(fmt.Sprintf("> 描述: %s\n", desc))
				}
			}

			if len(alerts) > 2 {
				sb.WriteString(fmt.Sprintf("...还有 %d 条\n", len(alerts)-2))
			}
			sb.WriteString("\n")
		}
	}

	// 已恢复告警（最多显示2条）
	if group.TotalResolved > 0 {
		sb.WriteString("**已恢复**\n\n")
		displayCount := minInt(2, len(group.ResolvedAlerts))

		for i := 0; i < displayCount; i++ {
			alert := group.ResolvedAlerts[i]
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}
			instanceText := alert.Instance
			if len(instanceText) > 40 {
				instanceText = instanceText[:37] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. %s - `%s`\n", i+1, summaryText, instanceText))
		}

		if len(group.ResolvedAlerts) > 2 {
			sb.WriteString(fmt.Sprintf("...还有 %d 条\n", len(group.ResolvedAlerts)-2))
		}
		sb.WriteString("\n")
	}

	// 操作链接
	sb.WriteString(fmt.Sprintf("[查看详情](%s)\n", f.PortalUrl))

	return sb.String()
}

// FormatAggregatedAlertForFeiShu 为飞书格式化聚合后的多级别告警消息
// 使用富文本卡片展示多级别告警
func (f *MessageFormatter) FormatAggregatedAlertForFeiShu(group *AggregatedAlertGroup) (title string, content [][]map[string]any) {
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := group.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	title = f.PortalName + " 告警通知"

	// 构建消息内容
	content = [][]map[string]any{
		{{"tag": "text", "text": "📊 告警概况"}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": fmt.Sprintf("项目: %s", projectDisplay)}},
		{{"tag": "text", "text": fmt.Sprintf("时间: %s", now)}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": "📈 状态统计"}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": fmt.Sprintf("🔴 触发中: %d条", group.TotalFiring)}},
		{{"tag": "text", "text": fmt.Sprintf("🟢 已恢复: %d条", group.TotalResolved)}},
		{{"tag": "text", "text": ""}},
	}

	// 按集群统计
	if len(group.ClusterStats) > 0 {
		content = append(content, []map[string]any{
			{"tag": "text", "text": "按集群统计:"},
		})
		for clusterName, stat := range group.ClusterStats {
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("• %s: %d条触发 / %d条恢复", clusterName, stat.FiringCount, stat.ResolvedCount)},
			})
		}
		content = append(content, []map[string]any{{"tag": "text", "text": ""}})
	}

	// 按级别统计
	if len(group.AlertsBySeverity) > 0 {
		content = append(content, []map[string]any{
			{"tag": "text", "text": "按级别统计:"},
		})
		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, sev := range severityOrder {
			if alerts, ok := group.AlertsBySeverity[sev]; ok && len(alerts) > 0 {
				content = append(content, []map[string]any{
					{"tag": "text", "text": fmt.Sprintf("• %s: %d条", sev, len(alerts))},
				})
			}
		}
		content = append(content, []map[string]any{{"tag": "text", "text": ""}})
	}

	// 告警详情 - 按级别分组显示
	content = append(content, []map[string]any{
		{"tag": "text", "text": "🚨 告警详情"},
	})
	content = append(content, []map[string]any{{"tag": "text", "text": ""}})

	// 按优先级显示各级别告警
	severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
	for _, severity := range severityOrder {
		alerts, ok := group.AlertsBySeverity[severity]
		if !ok || len(alerts) == 0 {
			continue
		}

		// 级别标题
		content = append(content, []map[string]any{
			{"tag": "text", "text": fmt.Sprintf("级别: %s", severity)},
		})
		content = append(content, []map[string]any{{"tag": "text", "text": ""}})

		// 每个级别最多显示10条
		displayCount := minInt(10, len(alerts))
		for i := 0; i < displayCount; i++ {
			alert := alerts[i]
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}

			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("%d. %s", i+1, summaryText)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 告警集群: %s", alert.ClusterName)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 告警实例: %s", alert.Instance)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 触发时间: %s", f.GetAlertFiredTime(alert))},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 持续时间: %s", f.FormatDuration(alert.Duration))},
			})

			// 当前值
			if value := alert.Annotations["value"]; value != "" {
				content = append(content, []map[string]any{
					{"tag": "text", "text": fmt.Sprintf("   • 当前值: %s", value)},
				})
			}

			// 描述
			desc := f.GetAlertDescription(alert)
			if desc != "暂无描述" && desc != "" {
				content = append(content, []map[string]any{
					{"tag": "text", "text": fmt.Sprintf("   • 描述: %s", desc)},
				})
			}

			content = append(content, []map[string]any{{"tag": "text", "text": ""}})
		}

		if len(alerts) > 10 {
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("...还有 %d 条 %s 告警未显示", len(alerts)-10, severity)},
			})
			content = append(content, []map[string]any{{"tag": "text", "text": ""}})
		}
	}

	// 已恢复告警
	if group.TotalResolved > 0 {
		content = append(content, []map[string]any{
			{"tag": "text", "text": fmt.Sprintf("✅ 已恢复告警 (%d条)", group.TotalResolved)},
		})
		content = append(content, []map[string]any{{"tag": "text", "text": ""}})

		displayCount := minInt(5, len(group.ResolvedAlerts))
		for i := 0; i < displayCount; i++ {
			alert := group.ResolvedAlerts[i]
			summaryText := alert.Annotations["summary"]
			if summaryText == "" {
				summaryText = alert.AlertName
			}

			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("%d. %s", i+1, summaryText)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 集群: %s", alert.ClusterName)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 实例: %s", alert.Instance)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 恢复时间: %s", resolvedTime)},
			})
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("   • 持续时长: %s", f.FormatDuration(alert.Duration))},
			})
			content = append(content, []map[string]any{{"tag": "text", "text": ""}})
		}

		if len(group.ResolvedAlerts) > 5 {
			content = append(content, []map[string]any{
				{"tag": "text", "text": fmt.Sprintf("...还有 %d 条已恢复告警未显示", len(group.ResolvedAlerts)-5)},
			})
			content = append(content, []map[string]any{{"tag": "text", "text": ""}})
		}
	}

	// 底部操作链接
	content = append(content, []map[string]any{
		{"tag": "a", "text": "🔗 查看详情", "href": f.PortalUrl},
	})

	return
}

// FormatAggregatedAlertForEmail 为邮件格式化聚合后的多级别告警消息
// 使用 HTML 表格展示所有级别的告警详情，无显示数量限制
func (f *MessageFormatter) FormatAggregatedAlertForEmail(group *AggregatedAlertGroup) (subject, body string) {
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := group.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	highestSeverity := f.getHighestSeverity(group.AlertsBySeverity)
	color := f.GetSeverityColor(highestSeverity)

	// 构建邮件主题
	subject = fmt.Sprintf("[%s] %s - %s 告警 (%d触发/%d恢复)",
		strings.ToUpper(highestSeverity),
		html.EscapeString(f.PortalName),
		html.EscapeString(projectDisplay),
		group.TotalFiring,
		group.TotalResolved)

	// 对用户输入进行 HTML 转义
	escapedPortalName := html.EscapeString(f.PortalName)
	escapedProjectDisplay := html.EscapeString(projectDisplay)
	escapedSeverity := html.EscapeString(strings.ToUpper(highestSeverity))

	body = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;line-height:1.6;color:#333;margin:0;padding:20px;background:#f5f5f5}
.email-container{max-width:900px;min-height:200px;max-height:800px;margin:0 auto;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.12)}
.header{background:%s;color:#fff;padding:28px 32px;text-align:center;border-bottom:4px solid rgba(0,0,0,0.1)}
.header h1{margin:0;font-size:22px;font-weight:700;letter-spacing:-0.5px}
.header p{margin:10px 0 0;opacity:0.95;font-size:14px;font-weight:500}
.content{padding:28px 32px;max-height:600px;overflow-y:auto}
.content::-webkit-scrollbar{width:8px}
.content::-webkit-scrollbar-track{background:#f1f1f1;border-radius:4px}
.content::-webkit-scrollbar-thumb{background:#888;border-radius:4px}
.content::-webkit-scrollbar-thumb:hover{background:#555}
.info-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-bottom:24px;padding:20px;background:linear-gradient(135deg,#f8f9fa 0%%,#e9ecef 100%%);border-radius:10px;border:1px solid #dee2e6}
.info-item{padding:12px;background:#fff;border-radius:8px;box-shadow:0 2px 4px rgba(0,0,0,0.05)}
.info-label{font-size:11px;color:#6c757d;margin-bottom:6px;text-transform:uppercase;letter-spacing:0.5px;font-weight:600}
.info-value{font-size:15px;font-weight:700;color:#212529}
.stats{display:grid;grid-template-columns:repeat(2,1fr);gap:16px;margin-bottom:28px}
.stat{padding:20px;border-radius:10px;text-align:center;box-shadow:0 2px 8px rgba(0,0,0,0.08);transition:transform 0.2s}
.stat.firing{background:linear-gradient(135deg,#fff5f5 0%%,#ffe5e5 100%%);border:2px solid #ff4d4f}
.stat.resolved{background:linear-gradient(135deg,#f6ffed 0%%,#d9f7be 100%%);border:2px solid #52c41a}
.stat-num{font-size:36px;font-weight:800;line-height:1.2;margin-bottom:8px}
.stat-num.firing{color:#cf1322}
.stat-num.resolved{color:#389e0d}
.stat-label{font-size:13px;color:#595959;font-weight:600;text-transform:uppercase;letter-spacing:0.5px}
.section{margin-bottom:24px}
.section-title{font-size:16px;font-weight:700;color:#262626;margin-bottom:16px;padding:12px 16px;background:#fafafa;border-left:4px solid %s;border-radius:4px}
.severity-section{margin-bottom:32px}
.severity-title{font-size:18px;font-weight:700;color:#262626;margin-bottom:16px;padding:14px 18px;background:#f0f0f0;border-left:6px solid %s;border-radius:6px}
.alert-list{display:flex;flex-direction:column;gap:16px}
.alert-item{padding:20px;background:#fafafa;border-radius:10px;border-left:5px solid %s;box-shadow:0 2px 6px rgba(0,0,0,0.06);transition:box-shadow 0.2s}
.alert-item:hover{box-shadow:0 4px 12px rgba(0,0,0,0.1)}
.alert-item.resolved{border-left-color:#52c41a;background:linear-gradient(135deg,#f9fff6 0%%,#f0f9ff 100%%)}
.alert-header{font-weight:700;color:#262626;margin-bottom:12px;font-size:16px;display:flex;align-items:center;gap:8px}
.alert-header::before{content:'🔴';font-size:14px}
.alert-item.resolved .alert-header::before{content:'✅'}
.alert-meta{font-size:13px;color:#595959;line-height:2;background:#fff;padding:12px;border-radius:6px;margin-bottom:12px}
.alert-meta-row{margin-bottom:6px;display:flex;align-items:baseline}
.alert-meta-label{display:inline-block;min-width:100px;color:#8c8c8c;font-weight:600;font-size:12px}
.alert-meta-value{flex:1;color:#262626;font-weight:500}
.alert-meta code{background:#f0f0f0;padding:3px 8px;border-radius:4px;font-family:'SF Mono',Monaco,Consolas,monospace;font-size:12px;color:#d73a49;border:1px solid #e1e4e8}
.alert-desc{margin-top:12px;padding:12px 16px;background:#fff;border-radius:6px;font-size:14px;color:#595959;line-height:1.8;border-left:3px solid #1890ff}
.cluster-stats{margin-bottom:20px;padding:16px;background:#f8f9fa;border-radius:8px}
.cluster-stat-item{padding:8px 12px;margin:4px 0;background:#fff;border-radius:4px;font-size:14px}
.btn{display:inline-block;background:%s;color:#fff;padding:14px 40px;text-decoration:none;border-radius:8px;font-weight:700;font-size:15px;margin-top:20px;box-shadow:0 4px 12px rgba(0,0,0,0.15);transition:all 0.3s}
.btn:hover{transform:translateY(-2px);box-shadow:0 6px 16px rgba(0,0,0,0.2)}
.footer{padding:20px 32px;background:#fafafa;text-align:center;font-size:12px;color:#8c8c8c;border-top:1px solid #e8e8e8}
.footer-warning{color:#ff4d4f;font-weight:700;margin-top:10px;font-size:13px}
@media (max-width:768px){
.email-container{margin:0;border-radius:0;max-height:none}
.content{padding:20px;max-height:none}
.info-grid{grid-template-columns:1fr;gap:12px}
.stats{grid-template-columns:1fr}
}
</style>
</head>
<body>
<div class="email-container">
<div class="header">
<h1>%s 告警通知</h1>
<p>%s</p>
</div>
<div class="content">
<div class="info-grid">
<div class="info-item"><div class="info-label">项目</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">级别</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">通知时间</div><div class="info-value">%s</div></div>
</div>
<div class="stats">
<div class="stat firing"><div class="stat-num firing">%d</div><div class="stat-label">触发告警</div></div>
<div class="stat resolved"><div class="stat-num resolved">%d</div><div class="stat-label">已恢复</div></div>
</div>`,
		color, color, color, color, color,
		escapedPortalName, now,
		escapedProjectDisplay, escapedSeverity, now,
		group.TotalFiring, group.TotalResolved)

	// 集群统计
	if len(group.ClusterStats) > 0 {
		body += `<div class="cluster-stats"><div class="section-title">按集群统计</div>`
		for clusterName, stat := range group.ClusterStats {
			escapedClusterName := html.EscapeString(clusterName)
			body += fmt.Sprintf(`<div class="cluster-stat-item">• %s: %d条触发 / %d条恢复</div>`,
				escapedClusterName, stat.FiringCount, stat.ResolvedCount)
		}
		body += `</div>`
	}

	// 按级别统计
	if len(group.AlertsBySeverity) > 0 {
		body += `<div class="cluster-stats"><div class="section-title">按级别统计</div>`
		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, sev := range severityOrder {
			if alerts, ok := group.AlertsBySeverity[sev]; ok && len(alerts) > 0 {
				body += fmt.Sprintf(`<div class="cluster-stat-item">• %s: %d条</div>`, sev, len(alerts))
			}
		}
		body += `</div>`
	}

	// 告警详情 - 按级别分组显示（显示所有告警）
	if group.TotalFiring > 0 {
		severityOrder := []string{"CRITICAL", "WARNING", "INFO"}
		for _, severity := range severityOrder {
			alerts, ok := group.AlertsBySeverity[severity]
			if !ok || len(alerts) == 0 {
				continue
			}

			severityColor := f.GetSeverityColor(strings.ToLower(severity))
			body += fmt.Sprintf(`<div class="severity-section">
<div class="severity-title" style="border-left-color:%s">🚨 级别: %s (%d条)</div>
<div class="alert-list">`, severityColor, severity, len(alerts))

			for i, alert := range alerts {
				firedTime := f.GetAlertFiredTime(alert)

				escapedSummary := html.EscapeString(alert.Annotations["summary"])
				if escapedSummary == "" {
					escapedSummary = html.EscapeString(alert.AlertName)
				}
				escapedClusterName := html.EscapeString(alert.ClusterName)
				escapedInstance := html.EscapeString(alert.Instance)
				escapedValue := html.EscapeString(alert.Annotations["value"])
				escapedDescription := html.EscapeString(alert.Annotations["description"])
				escapedAlertDesc := html.EscapeString(f.GetAlertDescription(alert))

				body += fmt.Sprintf(`<div class="alert-item">
<div class="alert-header">%d. %s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">告警集群</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">告警实例</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">触发时间</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">持续时间</span><span class="alert-meta-value">%s</span></div>`,
					i+1, escapedSummary, escapedClusterName, escapedInstance, firedTime, f.FormatDuration(alert.Duration))

				if escapedValue != "" {
					body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">当前值</span><span class="alert-meta-value">%s</span></div>`, escapedValue)
				}

				body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">重复次数</span><span class="alert-meta-value">%d 次</span></div>`, alert.RepeatCount)

				if escapedDescription != "" && escapedDescription != "暂无描述" {
					body += fmt.Sprintf(`<div class="alert-meta-row"><span class="alert-meta-label">告警详情</span><span class="alert-meta-value">%s</span></div>`, escapedDescription)
				}

				body += `</div>`

				if escapedAlertDesc != "暂无描述" && escapedAlertDesc != "" {
					body += fmt.Sprintf(`<div class="alert-desc">📝 %s</div>`, escapedAlertDesc)
				}

				body += `</div>`
			}

			body += `</div></div>`
		}
	}

	// 恢复通知（显示所有恢复）
	if group.TotalResolved > 0 {
		body += `<div class="section"><div class="section-title">✅ 已恢复告警</div><div class="alert-list">`

		for i, alert := range group.ResolvedAlerts {
			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			escapedSummary := html.EscapeString(alert.Annotations["summary"])
			if escapedSummary == "" {
				escapedSummary = html.EscapeString(alert.AlertName)
			}
			escapedClusterName := html.EscapeString(alert.ClusterName)
			escapedAlertName := html.EscapeString(alert.AlertName)
			escapedInstance := html.EscapeString(alert.Instance)

			body += fmt.Sprintf(`<div class="alert-item resolved">
<div class="alert-header">%d. %s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">告警集群</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">告警规则</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">告警实例</span><span class="alert-meta-value"><code>%s</code></span></div>
<div class="alert-meta-row"><span class="alert-meta-label">恢复时间</span><span class="alert-meta-value">%s</span></div>
<div class="alert-meta-row"><span class="alert-meta-label">持续时长</span><span class="alert-meta-value">%s</span></div>
</div>
</div>`, i+1, escapedSummary, escapedClusterName, escapedAlertName, escapedInstance, resolvedTime, f.FormatDuration(alert.Duration))
		}

		body += `</div></div>`
	}

	// 操作按钮和页脚
	body += fmt.Sprintf(`<div style="text-align:center;margin-top:32px"><a href="%s" class="btn">🔗 立即处理告警</a></div>
</div>
<div class="footer">
<div>此邮件由 %s 自动发送</div>
<div class="footer-warning">⚠️ 请及时处理告警，系统自动发出请勿回复</div>
</div>
</div>
</body>
</html>`, f.PortalUrl, escapedPortalName)

	return
}
