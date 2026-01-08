package notification

import (
	"fmt"
	"strings"
	"time"
)

// MessageFormatter 消息格式化器
type MessageFormatter struct {
	PortalName string
	PortalUrl  string
}

// NewMessageFormatter 创建消息格式化器
func NewMessageFormatter(portalName, portalUrl string) *MessageFormatter {
	return &MessageFormatter{
		PortalName: portalName,
		PortalUrl:  portalUrl,
	}
}

// AlertSummary 告警统计摘要
type AlertSummary struct {
	FiringCount    int
	ResolvedCount  int
	FiringAlerts   []*AlertInstance
	ResolvedAlerts []*AlertInstance
}

// AnalyzeAlerts 分析告警列表
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

// GetSeverityEmoji 获取级别对应的 Emoji
// TODO: 暂时取消 标题图标
func (f *MessageFormatter) GetSeverityEmoji(severity string) string {
	return ""
}

// GetSeverityColor 获取级别对应的颜色（带#前缀）
func (f *MessageFormatter) GetSeverityColor(severity string) string {
	switch strings.ToLower(severity) {
	case "info":
		return "#9e9e9e"
	case "warning":
		return "#ffc107"
	case "critical":
		return "#dc3545"
	default:
		return "#28a745"
	}
}

// GetSeverityColorHex 获取级别对应的颜色（不带#前缀）
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

// FormatDuration 格式化持续时间
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
func (f *MessageFormatter) GetAlertDescription(alert *AlertInstance) string {
	if summary, ok := alert.Annotations["summary"]; ok && summary != "" {
		return summary
	}
	if desc, ok := alert.Annotations["description"]; ok && desc != "" {
		return desc
	}
	if msg, ok := alert.Annotations["message"]; ok && msg != "" {
		return msg
	}
	return "暂无描述"
}

// GetAlertFiredTime 获取告警触发时间
func (f *MessageFormatter) GetAlertFiredTime(alert *AlertInstance) string {
	// 使用 StartsAt 字段作为告警触发时间
	return alert.StartsAt.Format("2006-01-02 15:04:05")
}

// FormatMarkdownForDingTalk 为钉钉格式化 Markdown 消息
func (f *MessageFormatter) FormatMarkdownForDingTalk(opts *AlertOptions, alerts []*AlertInstance) (title, content string) {
	summary := f.AnalyzeAlerts(alerts)
	emoji := f.GetSeverityEmoji(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	title = fmt.Sprintf("%s %s 告警通知", emoji, f.PortalName)

	var sb strings.Builder

	// 标题
	sb.WriteString(fmt.Sprintf("#### %s %s 告警通知\n\n", emoji, f.PortalName))
	sb.WriteString("---\n\n")
	// 基本信息 - 使用引用格式
	sb.WriteString(fmt.Sprintf("> **项目**: %s  \n", projectDisplay))
	sb.WriteString(fmt.Sprintf("> **集群**: %s  \n", opts.ClusterName))
	sb.WriteString(fmt.Sprintf("> **级别**: %s  \n", strings.ToUpper(opts.Severity)))
	sb.WriteString(fmt.Sprintf("> **时间**: %s  \n\n", now))

	// 统计信息
	sb.WriteString("---\n\n")
	if summary.FiringCount > 0 {
		sb.WriteString(fmt.Sprintf("**🚨 触发告警**: %d 条  \n", summary.FiringCount))
	}
	if summary.ResolvedCount > 0 {
		sb.WriteString(fmt.Sprintf("**✅ 已恢复**: %d 条  \n", summary.ResolvedCount))
	}
	sb.WriteString("\n")

	// 告警详情（前5条）
	if summary.FiringCount > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("**🚨 告警详情（前 %d 条）**\n\n", min(5, summary.FiringCount)))
		displayCount := min(5, len(summary.FiringAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 40 {
				instanceText = instanceText[:37] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. **%s**  \n", i+1, alert.Annotations["summary"]))
			sb.WriteString(fmt.Sprintf("   - 实例: `%s`  \n", instanceText))
			sb.WriteString(fmt.Sprintf("   - 触发时间: %s  \n", f.GetAlertFiredTime(alert)))
			sb.WriteString(fmt.Sprintf("   - 持续: %s | 重复: %d 次 | 触发阈值: %s  \n", f.FormatDuration(alert.Duration), alert.RepeatCount, alert.Annotations["value"]))
			sb.WriteString(fmt.Sprintf("   - 详情: `%s`  \n", alert.Annotations["description"]))
			desc := f.GetAlertDescription(alert)
			if len(desc) > 60 {
				desc = desc[:57] + "..."
			}
			sb.WriteString(fmt.Sprintf("   - 影响: %s  \n\n", desc))
		}
		if summary.FiringCount > 5 {
			sb.WriteString(fmt.Sprintf("*...还有 %d 条告警*\n\n", summary.FiringCount-5))
		}
	}

	// 恢复通知（前3条）
	if summary.ResolvedCount > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("**✅ 已恢复（前 %d 条）**\n\n", min(3, summary.ResolvedCount)))
		displayCount := min(3, len(summary.ResolvedAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 40 {
				instanceText = instanceText[:37] + "..."
			}
			sb.WriteString(fmt.Sprintf("%d. %s - `%s`  \n", i+1, alert.AlertName, instanceText))
		}
		sb.WriteString("\n")
	}

	// 链接
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("[🔗 立即处理](%s)", f.PortalUrl))

	content = sb.String()
	return
}

// FormatNotificationForDingTalk 为钉钉格式化通知消息
func (f *MessageFormatter) FormatNotificationForDingTalk(opts *NotificationOptions) (title, content string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	emoji := f.GetSeverityEmoji("notification")

	title = fmt.Sprintf("%s %s", emoji, opts.Title)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("#### %s %s %s 通知\n\n", emoji, f.PortalName, opts.Title))
	sb.WriteString("---\n\n")

	sb.WriteString(fmt.Sprintf("> **时间**: %s\n\n", now))
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("%s\n\n", opts.Content))
	sb.WriteString("\n\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("[🔗 前往控制台](%s)", f.PortalUrl))

	content = sb.String()
	return
}

// FormatMarkdownForWeChat 为企业微信格式化 Markdown 消息
// 注意：企业微信的markdown不支持分割线(---)，只支持有限的格式
func (f *MessageFormatter) FormatMarkdownForWeChat(opts *AlertOptions, alerts []*AlertInstance) string {
	summary := f.AnalyzeAlerts(alerts)
	emoji := f.GetSeverityEmoji(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	var sb strings.Builder

	// 标题
	sb.WriteString(fmt.Sprintf("### %s %s 告警通知\n\n", emoji, f.PortalName))

	// 基本信息 - 使用引用格式
	sb.WriteString(fmt.Sprintf("> **项目**: %s\n", projectDisplay))
	sb.WriteString(fmt.Sprintf("> **集群**: %s\n", opts.ClusterName))
	sb.WriteString(fmt.Sprintf("> **级别**: <font color=\"warning\">%s</font>\n", strings.ToUpper(opts.Severity)))
	sb.WriteString(fmt.Sprintf("> **时间**: %s\n\n", now))

	// 统计信息
	if summary.FiringCount > 0 {
		sb.WriteString(fmt.Sprintf("**🚨 触发告警**: <font color=\"warning\">%d</font> 条  \n", summary.FiringCount))
	}
	if summary.ResolvedCount > 0 {
		sb.WriteString(fmt.Sprintf("**✅ 已恢复**: <font color=\"info\">%d</font> 条  \n", summary.ResolvedCount))
	}
	sb.WriteString("\n")

	// 告警详情（前5条）
	if summary.FiringCount > 0 {
		sb.WriteString(fmt.Sprintf("**🚨 告警详情（前 %d 条）**\n", min(5, summary.FiringCount)))
		displayCount := min(5, len(summary.FiringAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 35 {
				instanceText = instanceText[:32] + "..."
			}
			sb.WriteString(fmt.Sprintf("\n%d. **%s**\n", i+1, alert.Annotations["summary"]))
			sb.WriteString(fmt.Sprintf("> 实例: `%s`\n", instanceText))
			sb.WriteString(fmt.Sprintf("> 触发时间: %s\n", f.GetAlertFiredTime(alert)))
			sb.WriteString(fmt.Sprintf("> 持续: <font color=\"warning\">%s</font> | 次数: <font color=\"warning\">%d</font> | 触发阈值: <font color=\"warning\">%s</font>\n", f.FormatDuration(alert.Duration), alert.RepeatCount, alert.Annotations["value"]))
			sb.WriteString(fmt.Sprintf("> 详情: `%s`\n", alert.Annotations["description"]))
		}
		if summary.FiringCount > 5 {
			sb.WriteString(fmt.Sprintf("\n<font color=\"comment\">...还有 %d 条告警</font>\n", summary.FiringCount-5))
		}
		sb.WriteString("\n")
	}

	// 恢复通知（前3条）
	if summary.ResolvedCount > 0 {
		sb.WriteString(fmt.Sprintf("**✅ 已恢复（前 %d 条）**\n", min(3, summary.ResolvedCount)))
		displayCount := min(3, len(summary.ResolvedAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 35 {
				instanceText = instanceText[:32] + "..."
			}
			sb.WriteString(fmt.Sprintf("\n%d. %s - `%s`", i+1, alert.AlertName, instanceText))
		}
		sb.WriteString("\n\n")
	}

	// 链接
	sb.WriteString(fmt.Sprintf("[🔗 立即处理](%s)\n", f.PortalUrl))

	return sb.String()
}

// FormatNotificationForWeChat 微信消息通知
func (f *MessageFormatter) FormatNotificationForWeChat(opts *NotificationOptions) string {
	now := time.Now().Format("2006-01-02 15:04:05")

	var sb strings.Builder
	emoji := f.GetSeverityEmoji("notification")

	sb.WriteString(fmt.Sprintf("### %s %s %s 通知\n\n", emoji, f.PortalName, opts.Title))

	sb.WriteString(fmt.Sprintf("> **时间**: %s\n\n", now))
	sb.WriteString(fmt.Sprintf("%s\n\n", opts.Content))
	sb.WriteString(fmt.Sprintf("[🔗 前往控制台](%s)\n", f.PortalUrl))

	return sb.String()
}

// FormatRichTextForFeiShu 为飞书格式化富文本消息
func (f *MessageFormatter) FormatRichTextForFeiShu(opts *AlertOptions, alerts []*AlertInstance) (title string, content [][]map[string]interface{}) {
	summary := f.AnalyzeAlerts(alerts)
	emoji := f.GetSeverityEmoji(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	title = fmt.Sprintf("%s %s 告警通知", emoji, f.PortalName)

	content = [][]map[string]interface{}{
		// 基本信息 - 每行一个信息
		{{"tag": "text", "text": fmt.Sprintf("📦 项目: %s", projectDisplay)}},
		{{"tag": "text", "text": fmt.Sprintf("🌐 集群: %s", opts.ClusterName)}},
		{{"tag": "text", "text": fmt.Sprintf("⚡ 级别: %s", strings.ToUpper(opts.Severity))}},
		{{"tag": "text", "text": fmt.Sprintf("🕐 时间: %s", now)}},
		{{"tag": "text", "text": ""}},

		// 统计信息标题
		{{"tag": "text", "text": ""}},
	}

	// 统计信息
	if summary.FiringCount > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("🚨 触发告警: %d 条", summary.FiringCount)},
		})
	}
	if summary.ResolvedCount > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("✅ 已恢复: %d 条", summary.ResolvedCount)},
		})
	}

	// 告警详情（前5条）
	if summary.FiringCount > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("🚨 告警详情（前 %d 条）", min(5, summary.FiringCount))},
		})

		displayCount := min(5, len(summary.FiringAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 35 {
				instanceText = instanceText[:32] + "..."
			}

			// 每个告警占多行
			content = append(content, []map[string]interface{}{})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("%d. %s", i+1, alert.Annotations["summary"])},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   实例: %s", instanceText)},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   触发时间: %s", f.GetAlertFiredTime(alert))},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   持续: %s  次数: %d  触发阈值: %s", f.FormatDuration(alert.Duration), alert.RepeatCount, alert.Annotations["value"])},
			})
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   详情: %s", alert.Annotations["description"])},
			})

			// 添加描述（如果有）
			desc := f.GetAlertDescription(alert)
			if desc != "暂无描述" {
				if len(desc) > 50 {
					desc = desc[:47] + "..."
				}
				content = append(content, []map[string]interface{}{
					{"tag": "text", "text": fmt.Sprintf("   影响: %s", desc)},
				})
			}
		}

		if summary.FiringCount > 5 {
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("   ...还有 %d 条告警", summary.FiringCount-5)},
			})
		}
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
	}

	// 恢复通知（前3条）
	if summary.ResolvedCount > 0 {
		content = append(content, []map[string]interface{}{
			{"tag": "text", "text": fmt.Sprintf("✅ 已恢复（前 %d 条）", min(3, summary.ResolvedCount))},
		})

		displayCount := min(3, len(summary.ResolvedAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			instanceText := alert.Instance
			if len(instanceText) > 35 {
				instanceText = instanceText[:32] + "..."
			}
			content = append(content, []map[string]interface{}{
				{"tag": "text", "text": fmt.Sprintf("%d. %s - %s", i+1, alert.AlertName, instanceText)},
			})
		}
		content = append(content, []map[string]interface{}{{"tag": "text", "text": ""}})
	}

	// 链接
	content = append(content, []map[string]interface{}{
		{"tag": "a", "text": "🔗 立即处理", "href": f.PortalUrl},
	})

	return
}

// FormatNotificationForFeiShu 为飞书格式化通知消息
func (f *MessageFormatter) FormatNotificationForFeiShu(opts *NotificationOptions) (title string, content [][]map[string]interface{}) {
	now := time.Now().Format("2006-01-02 15:04:05")
	emoji := f.GetSeverityEmoji("notification")

	title = fmt.Sprintf("%s %s %s 通知", emoji, f.PortalName, opts.Title)

	content = [][]map[string]interface{}{
		{{"tag": "text", "text": fmt.Sprintf("📢 %s %s 通知", f.PortalName, opts.Title)}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": fmt.Sprintf("🕐 时间: %s", now)}},
		{{"tag": "text", "text": ""}},
		{{"tag": "text", "text": opts.Content}},
		{{"tag": "text", "text": ""}},
		{{"tag": "a", "text": "🔗 前往控制台", "href": f.PortalUrl}},
	}

	return
}

// FormatHTMLForEmail 为邮件格式化 HTML 消息
func (f *MessageFormatter) FormatHTMLForEmail(opts *AlertOptions, alerts []*AlertInstance) (subject, body string) {
	summary := f.AnalyzeAlerts(alerts)
	emoji := f.GetSeverityEmoji(opts.Severity)
	color := f.GetSeverityColor(opts.Severity)
	now := time.Now().Format("2006-01-02 15:04:05")

	projectDisplay := opts.ProjectName
	if projectDisplay == "" {
		projectDisplay = "集群级"
	}

	subject = fmt.Sprintf("[%s] %s - %s 告警 (%d告警/%d恢复)",
		strings.ToUpper(opts.Severity), f.PortalName, opts.ClusterName, summary.FiringCount, summary.ResolvedCount)

	body = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<style>
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;line-height:1.6;color:#333;margin:0;padding:20px;background:#f5f5f5}
.card{max-width:600px;margin:0 auto;background:#fff;border-radius:12px;overflow:hidden;box-shadow:0 4px 12px rgba(0,0,0,0.1)}
.header{background:%s;color:#fff;padding:24px;text-align:center}
.header h1{margin:0;font-size:20px;font-weight:600}
.header p{margin:8px 0 0;opacity:0.9;font-size:14px}
.content{padding:24px}
.info{display:flex;flex-wrap:wrap;gap:16px;margin-bottom:20px;padding:16px;background:#f8f9fa;border-radius:8px}
.info-item{flex:1;min-width:120px}
.info-label{font-size:12px;color:#666;margin-bottom:4px}
.info-value{font-size:14px;font-weight:600;color:#333}
.stats{display:flex;gap:16px;margin-bottom:24px}
.stat{flex:1;padding:16px;border-radius:8px;text-align:center}
.stat.firing{background:#fff2f0;border:1px solid #ffccc7}
.stat.resolved{background:#f6ffed;border:1px solid #b7eb8f}
.stat-num{font-size:28px;font-weight:700}
.stat-num.firing{color:#ff4d4f}
.stat-num.resolved{color:#52c41a}
.stat-label{font-size:12px;color:#666;margin-top:4px}
.section{margin-bottom:20px}
.section-title{font-size:14px;font-weight:600;color:#333;margin-bottom:12px;padding-bottom:8px;border-bottom:2px solid #eee}
.alert-list{max-height:400px;overflow-y:auto;padding-right:8px}
.alert-item{padding:16px;margin-bottom:12px;background:#fafafa;border-radius:6px;border-left:4px solid %s}
.alert-item.resolved{border-left-color:#52c41a;background:#f9fff6}
.alert-header{font-weight:600;color:#333;margin-bottom:8px;font-size:15px}
.alert-meta{font-size:13px;color:#666;line-height:1.8}
.alert-meta-row{margin-bottom:4px}
.alert-meta-label{display:inline-block;width:80px;color:#888;font-weight:500}
.alert-meta code{background:#e8e8e8;padding:2px 6px;border-radius:3px;font-family:monospace;font-size:12px}
.alert-desc{margin-top:8px;padding:8px;background:#fff;border-radius:4px;font-size:13px;color:#555;line-height:1.6}
.more-info{text-align:center;padding:12px;color:#999;font-size:13px}
.btn{display:inline-block;background:%s;color:#fff;padding:12px 32px;text-decoration:none;border-radius:6px;font-weight:600;margin-top:16px}
.footer{padding:16px 24px;background:#fafafa;text-align:center;font-size:12px;color:#999}
.footer-warning{color:#ff4d4f;font-weight:600;margin-top:8px}
</style>
</head>
<body>
<div class="card">
<div class="header">
<h1>%s %s 告警通知</h1>
<p>%s</p>
</div>
<div class="content">
<div class="info">
<div class="info-item"><div class="info-label">📦 项目</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">🌐 集群</div><div class="info-value">%s</div></div>
<div class="info-item"><div class="info-label">⚡ 级别</div><div class="info-value">%s</div></div>
</div>
<div class="stats">
<div class="stat firing"><div class="stat-num firing">%d</div><div class="stat-label">🚨 触发告警</div></div>
<div class="stat resolved"><div class="stat-num resolved">%d</div><div class="stat-label">✅ 已恢复</div></div>
</div>`,
		color, color, color, emoji, f.PortalName, now, projectDisplay, opts.ClusterName, strings.ToUpper(opts.Severity), summary.FiringCount, summary.ResolvedCount)

	if summary.FiringCount > 0 {
		body += `<div class="section"><div class="section-title">🚨 告警详情</div><div class="alert-list">`

		// 最多展示5条，用户可以滚动查看
		displayCount := min(5, len(summary.FiringAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.FiringAlerts[i]
			firedTime := f.GetAlertFiredTime(alert)

			body += fmt.Sprintf(`<div class="alert-item">
<div class="alert-header">%s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">实例:</span> <code>%s</code></div>
<div class="alert-meta-row"><span class="alert-meta-label">触发时间:</span> %s</div>
<div class="alert-meta-row"><span class="alert-meta-label">持续时间:</span> %s</div>
<div class="alert-meta-row"><span class="alert-meta-label">告警阈值:</span> %s </div>
<div class="alert-meta-row"><span class="alert-meta-label">重复次数:</span> %d 次</div>
<div class="alert-meta-row"><span class="alert-meta-label">告警详情:</span> %s</div>
</div>
<div class="alert-desc">%s</div>
</div>`, alert.Annotations["summary"], alert.Instance, firedTime, f.FormatDuration(alert.Duration), alert.Annotations["value"], alert.RepeatCount, alert.Annotations["description"], f.GetAlertDescription(alert))
		}

		if summary.FiringCount > 5 {
			body += fmt.Sprintf(`<div class="more-info">...还有 %d 条告警</div>`, summary.FiringCount-5)
		}

		body += `</div></div>`
	}

	// 恢复通知 - 添加滚动支持
	if summary.ResolvedCount > 0 {
		body += `<div class="section"><div class="section-title">✅ 已恢复</div><div class="alert-list">`

		// 最多展示5条，与告警详情保持一致
		displayCount := min(5, len(summary.ResolvedAlerts))
		for i := 0; i < displayCount; i++ {
			alert := summary.ResolvedAlerts[i]
			resolvedTime := "-"
			if alert.ResolvedAt != nil {
				resolvedTime = alert.ResolvedAt.Format("15:04:05")
			}

			body += fmt.Sprintf(`<div class="alert-item resolved">
<div class="alert-header">%s</div>
<div class="alert-meta">
<div class="alert-meta-row"><span class="alert-meta-label">事件:</span> <code>%s</code></div>
<div class="alert-meta-row"><span class="alert-meta-label">实例:</span> <code>%s</code></div>
<div class="alert-meta-row"><span class="alert-meta-label">恢复时间:</span> %s</div>
</div>
</div>`, alert.Annotations["summary"], alert.AlertName, alert.Instance, resolvedTime)
		}

		if summary.ResolvedCount > 5 {
			body += fmt.Sprintf(`<div class="more-info">...还有 %d 条已恢复</div>`, summary.ResolvedCount-5)
		}

		body += `</div></div>`
	}

	body += fmt.Sprintf(`<div style="text-align:center"><a href="%s" class="btn">🔗 立即处理</a></div>
</div>
<div class="footer">
<div class="footer-warning">此邮件由 %s 自动发送，请及时处理告警</div>
<div style="margin-top:4px">系统自动发出，请勿回复</div>
</div>
</div>
</body>
</html>`, f.PortalUrl, f.PortalName)

	return
}

// FormatNotificationForEmail 为邮件格式化通知消息
func (f *MessageFormatter) FormatNotificationForEmail(opts *NotificationOptions) (subject, body string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	subject = fmt.Sprintf("[通知] %s - %s", f.PortalName, opts.Title)

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
			max-width: 900px;  /* 从 560px 增加到 900px */
			margin: 0 auto;
			background: #fff;
			border-radius: 12px;
			overflow: hidden;
			box-shadow: 0 4px 12px rgba(0,0,0,0.1);
		}
		.header {
			background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
			color: #fff;
			padding: 32px;  /* 增加内边距 */
			text-align: center;
		}
		.header h1 {
			margin: 0;
			font-size: 24px;  /* 增大字体 */
			font-weight: 600;
		}
		.header p {
			margin: 8px 0 0;
			opacity: 0.9;
			font-size: 14px;
		}
		.content {
			padding: 32px;  /* 增加内边距 */
		}
		.message {
			padding: 24px;  /* 增加内边距 */
			background: #f8f9fa;
			border-radius: 8px;
			margin-bottom: 24px;
			white-space: pre-wrap;
			font-size: 15px;  /* 稍微增大字体 */
			line-height: 1.8;
			min-height: 200px;  /* 设置最小高度，避免内容太少时过窄 */
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
		/* 响应式设计 */
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
			<h1>📢 %s</h1>
			<p>%s</p>
		</div>
		<div class="content">
			<div class="message">%s</div>
			<div style="text-align:center">
				<a href="%s" class="btn">🔗 前往控制台</a>
			</div>
		</div>
		<div class="footer">
			<div>此邮件由 %s 自动发送</div>
			<div class="footer-warning">系统自动发出，请勿回复</div>
		</div>
	</div>
</body>
</html>`, opts.Title, now, opts.Content, f.PortalUrl, f.PortalName)

	return
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
