package managerservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/svc"
	"github.com/yanshicheng/kube-nova/application/manager-rpc/pb"
	"github.com/yanshicheng/kube-nova/common/handler/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ReceiveAlertmanagerWebhookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewReceiveAlertmanagerWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReceiveAlertmanagerWebhookLogic {
	return &ReceiveAlertmanagerWebhookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ReceiveAlertmanagerWebhookLogic) ReceiveAlertmanagerWebhook(in *pb.ReceiveAlertmanagerWebhookReq) (*pb.ReceiveAlertmanagerWebhookResp, error) {
	if in.Webhook == nil {
		return nil, errorx.Msg("webhook 数据不能为空")
	}
	webhook := in.Webhook

	// 遍历所有告警
	for _, alert := range webhook.Alerts {
		if alert == nil {
			continue
		}

		// 解析必要字段
		fingerprint := alert.Fingerprint
		if fingerprint == "" {
			logx.Errorf("告警缺少 fingerprint，跳过")
			continue
		}

		status := alert.Status
		if status == "" {
			status = webhook.Status // 使用 webhook 级别的状态
		}

		// 提取标签信息
		clusterUuid := ""
		namespace := ""
		alertName := ""
		severity := "info"
		instance := ""

		if alert.Labels != nil {
			// 尝试从多个可能的 key 中获取 cluster_uuid
			if v, ok := alert.Labels["cluster_uuid"]; ok {
				clusterUuid = v
			} else if v, ok := alert.Labels["clusterUuid"]; ok {
				clusterUuid = v
			}

			if v, ok := alert.Labels["namespace"]; ok {
				namespace = v
			}
			if v, ok := alert.Labels["alertname"]; ok {
				alertName = v
			}
			if v, ok := alert.Labels["severity"]; ok {
				severity = v
			}
			if v, ok := alert.Labels["instance"]; ok {
				instance = v
			}
		}

		// 如果没有 instance，使用 fingerprint
		if instance == "" {
			instance = fingerprint
		}

		// 获取集群、项目、工作空间信息
		clusterInfo, projectInfo, workspaceInfo := l.resolveAlertContext(clusterUuid, namespace)

		// 序列化 labels 和 annotations 为 JSON
		labelsJSON := ""
		if alert.Labels != nil {
			labelsBytes, err := json.Marshal(alert.Labels)
			if err != nil {
				logx.Errorf("序列化 labels 失败: %v", err)
			} else {
				labelsJSON = string(labelsBytes)
			}
		}

		annotationsJSON := ""
		if alert.Annotations != nil {
			annotationsBytes, err := json.Marshal(alert.Annotations)
			if err != nil {
				logx.Errorf("序列化 annotations 失败: %v", err)
			} else {
				annotationsJSON = string(annotationsBytes)
			}
		}

		// 解析时间
		startsAt := time.Now()
		if alert.StartsAt != "" {
			parsedTime, err := time.Parse(time.RFC3339, alert.StartsAt)
			if err == nil {
				startsAt = parsedTime
			} else {
				logx.Errorf("解析 startsAt 失败: %v", err)
			}
		}

		// 修改：使用 sql.NullTime 处理 endsAt
		var endsAt sql.NullTime
		if alert.EndsAt != "" && alert.EndsAt != "0001-01-01T00:00:00Z" {
			parsedTime, err := time.Parse(time.RFC3339, alert.EndsAt)
			if err == nil {
				endsAt = sql.NullTime{Time: parsedTime, Valid: true}
			} else {
				logx.Errorf("解析 endsAt 失败: %v", err)
			}
		}

		// 查询是否已存在该告警实例
		existInstance, err := l.svcCtx.AlertInstancesModel.FindOneByFingerprintStatusIsDeleted(
			l.ctx,
			fingerprint,
			status,
			0,
		)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			logx.Errorf("查询告警实例失败: %v", err)
			continue
		}

		if existInstance != nil {
			// 更新已存在的告警实例
			existInstance.Status = status
			existInstance.Labels = labelsJSON
			existInstance.Annotations = annotationsJSON
			existInstance.EndsAt = endsAt // 修改：直接赋值 sql.NullTime
			existInstance.UpdatedBy = "alertmanager"

			// 更新集群、项目、工作空间信息
			existInstance.ClusterUuid = clusterUuid
			existInstance.ClusterName = clusterInfo.Name
			existInstance.ProjectId = projectInfo.Id
			existInstance.ProjectName = projectInfo.Name
			existInstance.WorkspaceId = workspaceInfo.Id
			existInstance.WorkspaceName = workspaceInfo.Name

			// 如果状态为 resolved，更新恢复时间和持续时长
			if status == "resolved" && endsAt.Valid {
				existInstance.ResolvedAt = endsAt // 修改：直接赋值 sql.NullTime
				duration := uint64(endsAt.Time.Sub(existInstance.StartsAt).Seconds())
				existInstance.Duration = duration
			}

			err = l.svcCtx.AlertInstancesModel.Update(l.ctx, existInstance)
			if err != nil {
				logx.Errorf("更新告警实例失败: %v", err)
				continue
			}

			logx.Infof("更新告警实例成功: fingerprint=%s, status=%s, cluster=%s, project=%s, workspace=%s",
				fingerprint, status, clusterInfo.Name, projectInfo.Name, workspaceInfo.Name)
		} else {
			// 创建新的告警实例
			newInstance := &model.AlertInstances{
				Instance:          instance,
				Fingerprint:       fingerprint,
				ClusterUuid:       clusterUuid,
				ClusterName:       clusterInfo.Name,
				ProjectId:         projectInfo.Id,
				ProjectName:       projectInfo.Name,
				WorkspaceId:       workspaceInfo.Id,
				WorkspaceName:     workspaceInfo.Name,
				AlertName:         alertName,
				Severity:          severity,
				Status:            status,
				Labels:            labelsJSON,
				Annotations:       annotationsJSON,
				GeneratorUrl:      alert.GeneratorURL,
				StartsAt:          startsAt,
				EndsAt:            endsAt,         // 修改：使用 sql.NullTime
				ResolvedAt:        sql.NullTime{}, // 修改：使用 sql.NullTime{}
				Duration:          0,
				NotifiedGroups:    "",
				NotificationCount: 0,
				LastNotifiedAt:    sql.NullTime{}, // 修改：使用 sql.NullTime{}
				CreatedBy:         "alertmanager",
				UpdatedBy:         "alertmanager",
				IsDeleted:         0,
			}

			// 如果状态为 resolved，设置恢复时间和持续时长
			if status == "resolved" && endsAt.Valid {
				newInstance.ResolvedAt = endsAt // 修改：直接赋值 sql.NullTime
				duration := uint64(endsAt.Time.Sub(startsAt).Seconds())
				newInstance.Duration = duration
			}

			_, err = l.svcCtx.AlertInstancesModel.Insert(l.ctx, newInstance)
			if err != nil {
				logx.Errorf("插入告警实例失败: %v", err)
				continue
			}

			logx.Infof("创建告警实例成功: fingerprint=%s, status=%s, cluster=%s, project=%s, workspace=%s",
				fingerprint, status, clusterInfo.Name, projectInfo.Name, workspaceInfo.Name)
		}
	}

	return &pb.ReceiveAlertmanagerWebhookResp{}, nil
}

// ClusterInfo 集群信息
type ClusterInfo struct {
	Name string
}

// ProjectInfo 项目信息
type ProjectInfo struct {
	Id   uint64
	Name string
}

// WorkspaceInfo 工作空间信息
type WorkspaceInfo struct {
	Id   uint64
	Name string
}

// resolveAlertContext 解析告警的上下文信息（集群、项目、工作空间）
func (l *ReceiveAlertmanagerWebhookLogic) resolveAlertContext(clusterUuid, namespace string) (ClusterInfo, ProjectInfo, WorkspaceInfo) {
	clusterInfo := ClusterInfo{}
	projectInfo := ProjectInfo{}
	workspaceInfo := WorkspaceInfo{}

	// 1. 查询集群信息
	if clusterUuid != "" {
		cluster, err := l.svcCtx.OnecClusterModel.FindOneByUuid(l.ctx, clusterUuid)
		if err != nil {
			if !errors.Is(err, model.ErrNotFound) {
				logx.Errorf("查询集群信息失败: cluster_uuid=%s, err=%v", clusterUuid, err)
			} else {
				logx.Errorf("集群不存在: cluster_uuid=%s", clusterUuid)
			}
			// TODO: 处理集群不存在的情况
			// 1. 是否需要记录未知集群的告警？
			// 2. 是否需要创建占位集群记录？
			// 3. 是否需要发送告警通知管理员？
		} else {
			clusterInfo.Name = cluster.Name
		}
	} else {
		// TODO: 告警中没有 cluster_uuid 标签
		// 这种情况可能的原因：
		// 1. Prometheus 配置中没有添加 cluster_uuid 标签
		// 2. 告警规则配置有误
		// 3. 某些全局告警不需要关联集群
		// 建议：检查 Prometheus 的 external_labels 配置
		logx.Errorf("告警缺少 cluster_uuid 标签")
	}

	// 2. 如果有 namespace 和 clusterUuid，查询工作空间和项目信息
	if namespace != "" && clusterUuid != "" {
		// 查询该集群下所有的项目集群关系
		projectClusters, err := l.svcCtx.OnecProjectClusterModel.SearchNoPage(
			l.ctx,
			"",
			false,
			"`cluster_uuid` = ?",
			clusterUuid,
		)
		if err != nil && !errors.Is(err, model.ErrNotFound) {
			logx.Errorf("查询项目集群关系失败: cluster_uuid=%s, err=%v", clusterUuid, err)
		} else if len(projectClusters) > 0 {
			// 遍历所有项目集群关系，查找匹配的工作空间
			for _, pc := range projectClusters {
				workspace, err := l.svcCtx.OnecProjectWorkspaceModel.FindOneByProjectClusterIdNamespace(
					l.ctx,
					pc.Id,
					namespace,
				)
				if err != nil {
					if !errors.Is(err, model.ErrNotFound) {
						logx.Errorf("查询工作空间失败: project_cluster_id=%d, namespace=%s, err=%v",
							pc.Id, namespace, err)
					}
					continue
				}

				// 找到匹配的工作空间
				workspaceInfo.Id = workspace.Id
				workspaceInfo.Name = workspace.Name
				projectInfo.Id = pc.ProjectId

				// 查询项目详细信息
				project, err := l.svcCtx.OnecProjectModel.FindOne(l.ctx, pc.ProjectId)
				if err != nil {
					logx.Errorf("查询项目信息失败: project_id=%d, err=%v", pc.ProjectId, err)
				} else {
					projectInfo.Name = project.Name
				}

				break // 找到后退出循环
			}

			if workspaceInfo.Id == 0 {
				// TODO: 该集群下找不到对应 namespace 的工作空间
				// 可能的原因：
				// 1. namespace 尚未在系统中注册为工作空间
				// 2. 工作空间已被删除但 namespace 仍存在
				// 3. 系统级 namespace（如 kube-system）未纳入管理
				// 建议：
				// 1. 记录这些告警便于后续分析
				// 2. 考虑是否需要自动创建工作空间
				// 3. 对于系统 namespace，可以关联到特殊的系统项目
				logx.Errorf("未找到工作空间: cluster_uuid=%s, namespace=%s", clusterUuid, namespace)
			}
		} else {
			// TODO: 该集群下没有关联任何项目
			// 可能的原因：
			// 1. 新集群刚添加，尚未分配给任何项目
			// 2. 集群配置有误
			// 建议：
			// 1. 告警仍然记录，但标记为未分配项目
			// 2. 通知管理员处理集群分配
			logx.Errorf("集群未关联任何项目: cluster_uuid=%s", clusterUuid)
		}
	} else {
		if namespace == "" {
			// TODO: 告警中没有 namespace 标签
			// 这种情况可能的原因：
			// 1. 集群级别的告警（如节点故障、网络问题）
			// 2. 告警规则配置有误
			// 3. Prometheus 抓取配置问题
			// 建议：
			// 1. 区分集群级告警和应用级告警
			// 2. 集群级告警可以不关联工作空间，只关联集群
			// 3. 应用级告警缺少 namespace 可能是配置错误
			logx.Errorf("告警缺少 namespace 标签，无法关联到工作空间和项目")
		}
	}

	return clusterInfo, projectInfo, workspaceInfo
}
