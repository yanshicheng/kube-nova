package notification

import (
	"context"
	"strings"
	"time"
)

// buildAggregatedAlertGroupFromAlerts 从告警列表构建聚合告警组
func buildAggregatedAlertGroupFromAlerts(alerts []*AlertInstance, projectName string) *AggregatedAlertGroup {
	if len(alerts) == 0 {
		return &AggregatedAlertGroup{
			ProjectName:      projectName,
			AlertsBySeverity: make(map[string][]*AlertInstance),
			ResolvedAlerts:   make([]*AlertInstance, 0),
			ClusterStats:     make(map[string]*ClusterStat),
		}
	}

	group := &AggregatedAlertGroup{
		ProjectID:        alerts[0].ProjectID,
		ProjectName:      projectName,
		AlertsBySeverity: make(map[string][]*AlertInstance),
		ResolvedAlerts:   make([]*AlertInstance, 0),
		ClusterStats:     make(map[string]*ClusterStat),
		TotalFiring:      0,
		TotalResolved:    0,
	}

	// 记录第一条和最后一条告警的时间
	var firstTime, lastTime time.Time

	for i, alert := range alerts {
		// 更新时间范围
		if i == 0 {
			firstTime = alert.StartsAt
			lastTime = alert.StartsAt
		} else {
			if alert.StartsAt.Before(firstTime) {
				firstTime = alert.StartsAt
			}
			if alert.StartsAt.After(lastTime) {
				lastTime = alert.StartsAt
			}
		}

		// 按状态分类
		if alert.Status == string(AlertStatusFiring) {
			// 按级别分组
			severity := strings.ToUpper(alert.Severity)
			group.AlertsBySeverity[severity] = append(group.AlertsBySeverity[severity], alert)
			group.TotalFiring++

			// 更新集群统计
			if stat, exists := group.ClusterStats[alert.ClusterName]; exists {
				stat.FiringCount++
			} else {
				group.ClusterStats[alert.ClusterName] = &ClusterStat{
					ClusterName:   alert.ClusterName,
					FiringCount:   1,
					ResolvedCount: 0,
				}
			}
		} else {
			// 已恢复的告警
			group.ResolvedAlerts = append(group.ResolvedAlerts, alert)
			group.TotalResolved++

			// 更新集群统计
			if stat, exists := group.ClusterStats[alert.ClusterName]; exists {
				stat.ResolvedCount++
			} else {
				group.ClusterStats[alert.ClusterName] = &ClusterStat{
					ClusterName:   alert.ClusterName,
					FiringCount:   0,
					ResolvedCount: 1,
				}
			}
		}
	}

	group.FirstAlertTime = firstTime
	group.LastAlertTime = lastTime

	return group
}

// sendAggregatedAlertToDingTalk 发送聚合告警到钉钉
func (m *manager) sendAggregatedAlertToDingTalk(ctx context.Context, client FullChannel, opts *AlertOptions, group *AggregatedAlertGroup) (*SendResult, error) {
	// 使用原有方法发送，渠道客户端内部会调用聚合格式化方法
	return client.SendPrometheusAlert(ctx, opts, m.flattenAlerts(group))
}

// sendAggregatedAlertToWeChat 发送聚合告警到企业微信
func (m *manager) sendAggregatedAlertToWeChat(ctx context.Context, client FullChannel, opts *AlertOptions, group *AggregatedAlertGroup) (*SendResult, error) {
	// 使用原有方法发送，渠道客户端内部会调用聚合格式化方法
	return client.SendPrometheusAlert(ctx, opts, m.flattenAlerts(group))
}

// sendAggregatedAlertToFeiShu 发送聚合告警到飞书
func (m *manager) sendAggregatedAlertToFeiShu(ctx context.Context, client FullChannel, opts *AlertOptions, group *AggregatedAlertGroup) (*SendResult, error) {
	// 使用原有方法发送，渠道客户端内部会调用聚合格式化方法
	return client.SendPrometheusAlert(ctx, opts, m.flattenAlerts(group))
}

// sendAggregatedAlertToEmail 发送聚合告警到邮件
func (m *manager) sendAggregatedAlertToEmail(ctx context.Context, client FullChannel, opts *AlertOptions, group *AggregatedAlertGroup) (*SendResult, error) {
	// 使用原有方法发送，渠道客户端内部会调用聚合格式化方法
	return client.SendPrometheusAlert(ctx, opts, m.flattenAlerts(group))
}

// flattenAlerts 将聚合告警组展平为告警列表
func (m *manager) flattenAlerts(group *AggregatedAlertGroup) []*AlertInstance {
	var alerts []*AlertInstance
	for _, severityAlerts := range group.AlertsBySeverity {
		alerts = append(alerts, severityAlerts...)
	}
	alerts = append(alerts, group.ResolvedAlerts...)
	return alerts
}
