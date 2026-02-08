package operator

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/yanshicheng/kube-nova/application/manager-rpc/internal/model"
	"github.com/yanshicheng/kube-nova/common/k8smanager/cluster"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
)

// FieldChange 字段变更记录
type FieldChange struct {
	Field    string      `json:"field"`    // 字段名
	OldValue interface{} `json:"oldValue"` // 旧值
	NewValue interface{} `json:"newValue"` // 新值
}

// clusterResourceStats 集群资源统计结构
type clusterResourceStats struct {
	TotalNodes  int64
	TotalCpu    float64
	TotalMemory float64
	TotalPods   int64
}

// SyncClusterVersion 同步集群版本信息（必须记录审计日志，不受enableAudit控制）
func (s *ClusterResourceSync) SyncClusterVersion(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	// 设置同步开始状态
	if err := s.updateClusterStatus(ctx, clusterUuid, 1); err != nil {
		s.Logger.WithContext(ctx).Error("更新集群同步状态失败: %v", err)
	}

	// 获取集群版本信息
	clusterClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Error("获取集群失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return err
	}

	versionInfo, err := clusterClient.GetVersionInfo()
	if err != nil {
		s.Logger.WithContext(ctx).Error("获取集群版本信息失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return err
	}

	// 查找集群记录
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Error("查询集群失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return err
	}

	// 记录变更前的值
	oldCluster := *onecCluster // 复制一份原始数据

	// 更新版本信息
	onecCluster.Version = versionInfo.Version
	onecCluster.GitCommit = versionInfo.GitCommit
	onecCluster.Platform = versionInfo.Platform
	onecCluster.ClusterCreatedAt = time.Unix(versionInfo.ClusterCreateAt, 0)
	onecCluster.VersionBuildAt = time.Unix(versionInfo.VersionBuildAt, 0)
	onecCluster.UpdatedBy = operator
	onecCluster.Status = 3 // 同步正常

	changes := s.detectClusterChanges(&oldCluster, onecCluster)

	// 如果有变更，记录审计日志（必须记录，不受enableAudit控制）
	if len(changes) > 0 {
		// 保存更新
		if err := s.ClusterModel.Update(ctx, onecCluster); err != nil {
			s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
			return fmt.Errorf("更新集群版本信息失败: %v", err)
		}

		// 必须记录审计日志
		auditContent := fmt.Sprintf("集群[%s]版本信息变更: %s", onecCluster.Name, s.buildChangeDescription(changes))
		s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "集群版本更新", 1)

		s.Logger.WithContext(ctx).Infof("成功同步集群 %s 的版本信息，有变更", onecCluster.Name)
	} else {
		// 无变更也需要更新状态为同步正常
		onecCluster.Status = 3
		if err := s.ClusterModel.Update(ctx, onecCluster); err != nil {
			s.Logger.WithContext(ctx).Error("更新集群状态失败: %v", err)
		}
		s.Logger.WithContext(ctx).Infof("集群 %s 的版本信息无变化", onecCluster.Name)
	}

	return nil
}

// SyncClusterResource 同步集群资源信息（必须记录审计日志，不受enableAudit控制）
func (s *ClusterResourceSync) SyncClusterResource(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	// 设置同步开始状态
	if err := s.updateClusterStatus(ctx, clusterUuid, 1); err != nil {
		s.Logger.WithContext(ctx).Error("更新集群同步状态失败: %v", err)
	}

	// 获取集群名称
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Error("查询集群失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2)
		return err
	}

	stats, err := s.calculateClusterResourceStats(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Error("计算集群资源统计失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return err
	}

	existingResource, err := s.ClusterResourceModel.FindOneByClusterUuid(ctx, clusterUuid)
	if err != nil {
		// 如果不存在则创建
		if errors.Is(err, model.ErrNotFound) {
			existingResource = &model.OnecClusterResource{
				ClusterUuid:             clusterUuid,
				StoragePhysicalCapacity: 9999, // 初始设置存储资源
				MemPhysicalCapacity:     stats.TotalMemory,
				PodsPhysicalCapacity:    stats.TotalPods,
				CpuPhysicalCapacity:     stats.TotalCpu,
			}
			_, err = s.ClusterResourceModel.Insert(ctx, existingResource)
			if err != nil {
				s.Logger.WithContext(ctx).Error("创建集群资源失败: %v", err)
				s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
				return err
			}

			// 必须记录资源新增审计日志
			auditContent := fmt.Sprintf("集群[%s]资源初始化: CPU=%.0fC, 内存=%.0fGi, Pod=%d",
				onecCluster.Name, stats.TotalCpu, stats.TotalMemory/1024/1024, stats.TotalPods)
			s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "集群资源新增", 1)

			// 更新状态为同步正常
			s.updateClusterStatus(ctx, clusterUuid, 3)
		} else {
			s.Logger.WithContext(ctx).Error("查询集群资源失败: %v", err)
			s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
			return err
		}
	} else {
		// 记录变更前的值
		oldResource := *existingResource

		// 更新资源信息（不更新storage_capacity）
		existingResource.CpuPhysicalCapacity = float64(stats.TotalCpu)
		existingResource.MemPhysicalCapacity = stats.TotalMemory
		existingResource.PodsPhysicalCapacity = stats.TotalPods

		// 检测资源变更
		changes := s.detectResourceChanges(&oldResource, existingResource)
		if len(changes) > 0 {
			// 更新资源记录
			if err := s.ClusterResourceModel.Update(ctx, existingResource); err != nil {
				s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
				return fmt.Errorf("更新集群资源记录失败: %v", err)
			}

			// 必须记录资源更新审计日志
			auditContent := fmt.Sprintf("集群[%s]资源变更: %s", onecCluster.Name, s.buildChangeDescription(changes))
			s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "集群资源更新", 1)

			s.Logger.WithContext(ctx).Infof("成功同步集群 %s 的资源信息，有变更", onecCluster.Name)
		} else {
			s.Logger.WithContext(ctx).Infof("集群 %s 的资源信息无变化", onecCluster.Name)
		}

		// 更新状态为同步正常
		s.updateClusterStatus(ctx, clusterUuid, 3)
	}

	return nil
}

// SyncClusterNetwork 同步集群网络配置信息（必须记录审计日志，不受enableAudit控制）
func (s *ClusterResourceSync) SyncClusterNetwork(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	s.Logger.WithContext(ctx).Infof("开始同步集群网络配置, clusterUuid: %s, operator: %s", clusterUuid, operator)

	// 设置同步开始状态
	if err := s.updateClusterStatus(ctx, clusterUuid, 1); err != nil {
		s.Logger.WithContext(ctx).Error("更新集群同步状态失败: %v", err)
	}

	//  验证集群是否存在
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return fmt.Errorf("查询集群失败: %v", err)
	}

	s.Logger.WithContext(ctx).Infof("集群信息: name=%s, uuid=%s", onecCluster.Name, onecCluster.Uuid)

	// 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败, clusterUuid: %s, error: %v", clusterUuid, err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("集群[%s]网络配置同步失败: 无法连接到集群", onecCluster.Name), "网络配置同步", 2)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 获取网络配置信息
	s.Logger.WithContext(ctx).Infof("开始检测集群 %s 的网络配置", onecCluster.Name)
	networkInfo, err := k8sClient.GetNetworkInfo()
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取集群网络信息失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("集群[%s]网络配置同步失败: %v", onecCluster.Name, err), "网络配置同步", 2)
		return fmt.Errorf("获取集群网络信息失败: %v", err)
	}

	// 查询数据库中是否已存在网络配置
	existingNetwork, err := s.ClusterNetwork.FindOneByClusterUuid(ctx, clusterUuid)

	if err != nil {
		// 如果不存在则创建
		if errors.Is(err, model.ErrNotFound) {
			s.Logger.WithContext(ctx).Infof("集群网络配置不存在，创建新记录")

			// 转换为数据库模型
			clusterNetwork := s.buildClusterNetworkModel(clusterUuid, *networkInfo, operator)

			// 插入数据库
			_, err = s.ClusterNetwork.Insert(ctx, clusterNetwork)
			if err != nil {
				s.Logger.WithContext(ctx).Errorf("创建集群网络配置失败: %v", err)
				s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
				// 必须记录失败审计日志
				s.writeClusterAuditLog(ctx, clusterUuid, operator,
					fmt.Sprintf("集群[%s]网络配置创建失败: %v", onecCluster.Name, err), "网络配置同步", 2)
				return fmt.Errorf("创建集群网络配置失败: %v", err)
			}

			// 必须记录新增审计日志
			auditContent := s.buildNetworkCreationAudit(onecCluster.Name, networkInfo)
			s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "网络配置新增", 1)

			// 更新状态为同步正常
			s.updateClusterStatus(ctx, clusterUuid, 3)
			s.Logger.WithContext(ctx).Infof("成功创建集群 %s 的网络配置", onecCluster.Name)
			return nil
		}

		s.Logger.WithContext(ctx).Errorf("查询集群网络配置失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
		return fmt.Errorf("查询集群网络配置失败: %v", err)
	}

	// 记录变更前的值
	oldNetwork := *existingNetwork

	// 更新网络配置
	s.updateNetworkModel(existingNetwork, *networkInfo, operator)

	// 检测网络配置变更
	changes := s.detectNetworkChanges(&oldNetwork, existingNetwork)

	if len(changes) > 0 {
		// 有变更，更新数据库
		if err := s.ClusterNetwork.Update(ctx, existingNetwork); err != nil {
			s.Logger.WithContext(ctx).Errorf("更新集群网络配置失败: %v", err)
			s.updateClusterStatus(ctx, clusterUuid, 2) // 同步异常
			// 必须记录失败审计日志
			s.writeClusterAuditLog(ctx, clusterUuid, operator,
				fmt.Sprintf("集群[%s]网络配置更新失败: %v", onecCluster.Name, err), "网络配置同步", 2)
			return fmt.Errorf("更新集群网络配置失败: %v", err)
		}

		// 必须记录网络配置更新审计日志
		auditContent := s.buildNetworkUpdateAudit(onecCluster.Name, changes)
		s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "网络配置更新", 1)

		s.Logger.WithContext(ctx).Infof("成功同步集群 %s 的网络配置，有变更: %d 项", onecCluster.Name, len(changes))
	} else {
		s.Logger.WithContext(ctx).Infof("集群 %s 的网络配置无变化", onecCluster.Name)
	}

	// 更新状态为同步正常
	s.updateClusterStatus(ctx, clusterUuid, 3)

	return nil
}

// SyncClusterNodes 同步集群节点信息（必须记录审计日志，不受enableAudit控制）
func (s *ClusterResourceSync) SyncClusterNodes(ctx context.Context, clusterUuid string, operator string, enableAudit bool) error {
	startTime := time.Now()
	s.Logger.WithContext(ctx).Infof("开始同步集群节点信息, clusterUuid: %s, operator: %s", clusterUuid, operator)

	// 设置同步开始状态
	if err := s.updateClusterStatus(ctx, clusterUuid, 1); err != nil {
		s.Logger.WithContext(ctx).Error("更新集群同步状态失败: %v", err)
	}

	// 验证集群是否存在
	onecCluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("查询集群失败, clusterUuid: %s, error: %v", clusterUuid, err)
		s.updateClusterStatus(ctx, clusterUuid, 2)
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("节点同步失败: 集群不存在或查询失败, error: %v", err), "节点同步", 2)
		return fmt.Errorf("查询集群失败: %v", err)
	}

	// 获取 K8s 客户端
	k8sClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取K8s客户端失败, clusterUuid: %s, error: %v", clusterUuid, err)
		s.updateClusterStatus(ctx, clusterUuid, 2)
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("集群[%s]节点同步失败: 无法连接到集群, error: %v", onecCluster.Name, err), "节点同步", 2)
		return fmt.Errorf("获取K8s客户端失败: %v", err)
	}

	// 获取集群所有节点列表
	nodeListReq := &types.NodeListRequest{
		Page:     1,
		PageSize: 1000,
		IsAsc:    true,
	}
	nodeListResp, err := k8sClient.Node().List(nodeListReq)
	if err != nil {
		s.Logger.WithContext(ctx).Errorf("获取集群节点列表失败: %v", err)
		s.updateClusterStatus(ctx, clusterUuid, 2)
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("集群[%s]节点同步失败: 获取K8s节点列表失败, error: %v", onecCluster.Name, err), "节点同步", 2)
		return fmt.Errorf("获取集群节点列表失败: %v", err)
	}

	// 检查返回结果是否为空
	if nodeListResp == nil {
		s.Logger.WithContext(ctx).Errorf("获取集群节点列表返回为空")
		s.updateClusterStatus(ctx, clusterUuid, 2)
		// 必须记录失败审计日志
		s.writeClusterAuditLog(ctx, clusterUuid, operator,
			fmt.Sprintf("集群[%s]节点同步失败: K8s返回节点列表为空", onecCluster.Name), "节点同步", 2)
		return fmt.Errorf("获取集群节点列表返回为空")
	}

	s.Logger.WithContext(ctx).Infof("从K8s获取到 %d 个节点", len(nodeListResp.Items))

	// 获取数据库中该集群的所有节点
	existingNodes, err := s.ClusterNodeModel.SearchNoPage(ctx, "", true, "`cluster_uuid` = ?", clusterUuid)
	if err != nil {
		if !errors.Is(err, model.ErrNotFound) {
			s.Logger.WithContext(ctx).Errorf("查询数据库节点列表失败: %v", err)
			s.updateClusterStatus(ctx, clusterUuid, 2)
			// 必须记录失败审计日志
			s.writeClusterAuditLog(ctx, clusterUuid, operator,
				fmt.Sprintf("集群[%s]节点同步失败: 查询数据库节点失败, error: %v", onecCluster.Name, err), "节点同步", 2)
			return fmt.Errorf("查询数据库节点列表失败: %v", err)
		}
		existingNodes = []*model.OnecClusterNode{}
	}

	if existingNodes == nil {
		existingNodes = []*model.OnecClusterNode{}
	}

	// 构建映射
	existingNodeMap := make(map[string]*model.OnecClusterNode)
	for _, node := range existingNodes {
		if node != nil && node.NodeUuid != "" {
			existingNodeMap[node.NodeUuid] = node
		}
	}

	k8sNodeMap := make(map[string]*types.NodeInfo)
	for _, nodeInfo := range nodeListResp.Items {
		if nodeInfo != nil && nodeInfo.NodeUuid != "" {
			k8sNodeMap[nodeInfo.NodeUuid] = nodeInfo
		}
	}

	var (
		addedCount    int
		updatedCount  int
		deletedCount  int
		failedCount   int
		addedNodes    []string
		updatedNodes  []string
		deletedNodes  []string
		failedNodes   []string
		updateDetails []string // 记录更新详情
	)

	// 处理新增和更新的节点
	for _, nodeInfo := range nodeListResp.Items {
		if nodeInfo == nil {
			continue
		}

		existingNode, exists := existingNodeMap[nodeInfo.NodeUuid]

		if !exists {
			// 新增节点
			newNode := s.buildClusterNodeModel(clusterUuid, nodeInfo, operator)
			_, err := s.ClusterNodeModel.Insert(ctx, newNode)
			if err != nil {
				s.Logger.WithContext(ctx).Errorf("新增节点失败, nodeName: %s, error: %v", nodeInfo.NodeName, err)
				failedCount++
				failedNodes = append(failedNodes, fmt.Sprintf("%s(新增失败)", nodeInfo.NodeName))
				// 必须记录单个节点新增失败审计日志
				s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
					"ADD_FAILED", nodeInfo.NodeName, nil, 2)
				continue
			}
			addedCount++
			addedNodes = append(addedNodes, nodeInfo.NodeName)
			s.Logger.WithContext(ctx).Infof("新增节点: %s", nodeInfo.NodeName)

			// 必须记录单个节点新增审计日志
			s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
				"ADD", nodeInfo.NodeName, nil, 1)
		} else {
			// 检测节点变更
			oldNode := *existingNode
			s.updateClusterNodeModel(existingNode, nodeInfo, operator)

			changes := s.detectNodeChanges(&oldNode, existingNode)
			if len(changes) > 0 {
				if err := s.ClusterNodeModel.Update(ctx, existingNode); err != nil {
					s.Logger.WithContext(ctx).Errorf("更新节点失败, nodeName: %s, error: %v", nodeInfo.NodeName, err)
					failedCount++
					failedNodes = append(failedNodes, fmt.Sprintf("%s(更新失败)", nodeInfo.NodeName))
					// 必须记录单个节点更新失败审计日志
					s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
						"UPDATE_FAILED", nodeInfo.NodeName, changes, 2)
					continue
				}
				updatedCount++
				updatedNodes = append(updatedNodes, nodeInfo.NodeName)

				// 记录更新详情
				changeDesc := s.buildChangeDescription(changes)
				updateDetails = append(updateDetails, fmt.Sprintf("%s: %s", nodeInfo.NodeName, changeDesc))

				s.Logger.WithContext(ctx).Infof("更新节点: %s, 变更: %d 项", nodeInfo.NodeName, len(changes))

				// 必须记录单个节点更新审计日志（包含详细变更）
				s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
					"UPDATE", nodeInfo.NodeName, changes, 1)
			}
		}
	}

	// 处理已删除的节点
	for nodeUuid, existingNode := range existingNodeMap {
		if existingNode == nil {
			continue
		}
		if _, exists := k8sNodeMap[nodeUuid]; !exists {
			// 软删除节点
			if err := s.ClusterNodeModel.Delete(ctx, existingNode.Id); err != nil {
				s.Logger.WithContext(ctx).Errorf("删除节点失败, nodeName: %s, error: %v", existingNode.Name, err)
				failedCount++
				failedNodes = append(failedNodes, fmt.Sprintf("%s(删除失败)", existingNode.Name))
				// 必须记录单个节点删除失败审计日志
				s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
					"DELETE_FAILED", existingNode.Name, nil, 2)
				continue
			}
			deletedCount++
			deletedNodes = append(deletedNodes, existingNode.Name)
			s.Logger.WithContext(ctx).Infof("删除节点: %s", existingNode.Name)

			// 必须记录单个节点删除审计日志
			s.writeNodeChangeAuditLog(ctx, clusterUuid, onecCluster.Name, operator,
				"DELETE", existingNode.Name, nil, 1)
		}
	}

	// 计算耗时
	duration := time.Since(startTime)

	// 必须记录汇总审计日志
	status := int64(1)
	var auditContent string

	if addedCount == 0 && updatedCount == 0 && deletedCount == 0 && failedCount == 0 {
		// 无任何变化
		auditContent = ""
	} else {
		// 有变化
		auditContent = s.buildNodeSyncSummaryAudit(onecCluster.Name, addedCount, updatedCount, deletedCount, failedCount,
			addedNodes, updatedNodes, deletedNodes, failedNodes, duration)

		if failedCount > 0 {
			status = 2 // 部分失败
		}
	}
	if auditContent != "" {
		s.writeClusterAuditLog(ctx, clusterUuid, operator, auditContent, "节点同步完成", status)
	}

	// 更新状态
	if failedCount > 0 {
		s.updateClusterStatus(ctx, clusterUuid, 2) // 部分失败
	} else {
		s.updateClusterStatus(ctx, clusterUuid, 3) // 同步正常
	}

	s.Logger.WithContext(ctx).Infof("集群[%s]节点同步完成: 新增=%d, 更新=%d, 删除=%d, 失败=%d, 耗时=%v",
		onecCluster.Name, addedCount, updatedCount, deletedCount, failedCount, duration)

	if failedCount > 0 {
		return fmt.Errorf("节点同步部分失败: 新增=%d, 更新=%d, 删除=%d, 失败=%d", addedCount, updatedCount, deletedCount, failedCount)
	}

	return nil
}

// updateClusterStatus 更新集群同步状态
// status: 1-同步中, 2-同步异常, 3-同步正常
func (s *ClusterResourceSync) updateClusterStatus(ctx context.Context, clusterUuid string, status int64) error {
	cluster, err := s.ClusterModel.FindOneByUuid(ctx, clusterUuid)
	if err != nil {
		return fmt.Errorf("查询集群失败: %v", err)
	}

	cluster.Status = status
	cluster.LastSyncAt = time.Now()

	if err := s.ClusterModel.Update(ctx, cluster); err != nil {
		return fmt.Errorf("更新集群状态失败: %v", err)
	}

	return nil
}

type ClusterNetworkInfo struct {
	ClusterCIDR       string
	ServiceCIDR       string
	NodeCIDRMaskSize  int
	DNSDomain         string
	DNSServiceIP      string
	DNSProvider       string
	CNIPlugin         string
	CNIVersion        string
	ProxyMode         string
	IngressController string
	IngressClass      string
	IPv6Enabled       bool
	DualStackEnabled  bool
	MTUSize           int
	EnableNodePort    bool
	NodePortRange     string
}

// buildClusterNetworkModel 构建集群网络数据库模型
func (s *ClusterResourceSync) buildClusterNetworkModel(clusterUuid string, networkInfo cluster.ClusterNetworkInfo, operator string) *model.OnecClusterNetwork {

	// bool 返回 init64
	dualStackEnabled := int64(0)
	if networkInfo.DualStackEnabled {
		dualStackEnabled = 1
	}
	ipv6Enabled := int64(0)
	if networkInfo.IPv6Enabled {
		ipv6Enabled = 1
	}
	return &model.OnecClusterNetwork{
		ClusterUuid:       clusterUuid,
		ClusterCidr:       s.getNetworkStringField(networkInfo, "ClusterCIDR"),
		ServiceCidr:       s.getNetworkStringField(networkInfo, "ServiceCIDR"),
		NodeCidrMaskSize:  int64(s.getNetworkIntField(networkInfo, "NodeCIDRMaskSize")),
		DnsDomain:         s.getNetworkStringField(networkInfo, "DNSDomain"),
		DnsServiceIp:      s.getNetworkStringField(networkInfo, "DNSServiceIP"),
		DnsProvider:       s.getNetworkStringField(networkInfo, "DNSProvider"),
		CniPlugin:         s.getNetworkStringField(networkInfo, "CNIPlugin"),
		CniVersion:        s.getNetworkStringField(networkInfo, "CNIVersion"),
		ProxyMode:         s.getNetworkStringField(networkInfo, "ProxyMode"),
		IngressController: s.getNetworkStringField(networkInfo, "IngressController"),
		IngressClass:      s.getNetworkStringField(networkInfo, "IngressClass"),
		Ipv6Enabled:       ipv6Enabled,
		DualStackEnabled:  dualStackEnabled,
		MtuSize:           int64(s.getNetworkIntField(networkInfo, "MTUSize")),
		NodePortRange:     s.getNetworkStringField(networkInfo, "NodePortRange"),
		CreatedBy:         operator,
		UpdatedBy:         operator,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		IsDeleted:         0,
	}

}

// updateNetworkModel 更新网络配置模型
func (s *ClusterResourceSync) updateNetworkModel(network *model.OnecClusterNetwork, networkInfo cluster.ClusterNetworkInfo, operator string) {
	ipV6 := int64(0)
	if networkInfo.IPv6Enabled {
		ipV6 = 1
	}
	dualStackEnabled := int64(0)
	if networkInfo.DualStackEnabled {
		dualStackEnabled = 1
	}
	network.ClusterCidr = s.getNetworkStringField(networkInfo, "ClusterCIDR")
	network.ServiceCidr = s.getNetworkStringField(networkInfo, "ServiceCIDR")
	network.NodeCidrMaskSize = int64(s.getNetworkIntField(networkInfo, "NodeCIDRMaskSize"))
	network.DnsDomain = s.getNetworkStringField(networkInfo, "DNSDomain")
	network.DnsServiceIp = s.getNetworkStringField(networkInfo, "DNSServiceIP")
	network.DnsProvider = s.getNetworkStringField(networkInfo, "DNSProvider")
	network.CniPlugin = s.getNetworkStringField(networkInfo, "CNIPlugin")
	network.CniVersion = s.getNetworkStringField(networkInfo, "CNIVersion")
	network.ProxyMode = s.getNetworkStringField(networkInfo, "ProxyMode")
	network.IngressController = s.getNetworkStringField(networkInfo, "IngressController")
	network.IngressClass = s.getNetworkStringField(networkInfo, "IngressClass")
	network.Ipv6Enabled = ipV6
	network.DualStackEnabled = dualStackEnabled
	network.MtuSize = int64(s.getNetworkIntField(networkInfo, "MTUSize"))
	network.NodePortRange = s.getNetworkStringField(networkInfo, "NodePortRange")
	network.UpdatedBy = operator
	network.UpdatedAt = time.Now()
}

// 使用反射从网络信息结构中获取字段值
func (s *ClusterResourceSync) getNetworkStringField(networkInfo interface{}, fieldName string) string {
	val := reflect.ValueOf(networkInfo)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return ""
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return ""
	}

	if field.Kind() == reflect.String {
		return field.String()
	}

	return ""
}

func (s *ClusterResourceSync) getNetworkIntField(networkInfo interface{}, fieldName string) int {
	val := reflect.ValueOf(networkInfo)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return 0
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return 0
	}

	if field.Kind() == reflect.Int || field.Kind() == reflect.Int64 {
		return int(field.Int())
	}

	return 0
}

func (s *ClusterResourceSync) getNetworkBoolField(networkInfo interface{}, fieldName string) bool {
	val := reflect.ValueOf(networkInfo)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false
	}

	field := val.FieldByName(fieldName)
	if !field.IsValid() {
		return false
	}

	if field.Kind() == reflect.Bool {
		return field.Bool()
	}

	return false
}

// detectNetworkChanges 检测网络配置变更
func (s *ClusterResourceSync) detectNetworkChanges(oldNetwork, newNetwork *model.OnecClusterNetwork) []FieldChange {
	var changes []FieldChange

	// 定义需要检测的字段映射（字段名 -> 中文名称）
	fieldsToCheck := map[string]string{
		"ClusterCidr":       "集群网络CIDR",
		"ServiceCidr":       "服务网络CIDR",
		"NodeCidrMaskSize":  "节点CIDR掩码",
		"DnsDomain":         "DNS域名",
		"DnsServiceIp":      "DNS服务IP",
		"DnsProvider":       "DNS提供者",
		"CniPlugin":         "CNI插件",
		"CniVersion":        "CNI版本",
		"ProxyMode":         "代理模式",
		"IngressController": "Ingress控制器",
		"IngressClass":      "Ingress类",
		"Ipv6Enabled":       "IPv6启用",
		"DualStackEnabled":  "双栈网络",
		"MtuSize":           "MTU大小",
		"EnableNodePort":    "NodePort启用",
		"NodePortRange":     "NodePort范围",
	}

	oldVal := reflect.ValueOf(oldNetwork).Elem()
	newVal := reflect.ValueOf(newNetwork).Elem()

	for fieldName, fieldDesc := range fieldsToCheck {
		oldField := oldVal.FieldByName(fieldName)
		newField := newVal.FieldByName(fieldName)

		if !oldField.IsValid() || !newField.IsValid() {
			continue
		}

		// 比较字段值
		if !s.compareFieldValues(oldField.Interface(), newField.Interface()) {
			changes = append(changes, FieldChange{
				Field:    fieldDesc,
				OldValue: s.formatNetworkValue(oldField.Interface(), fieldName),
				NewValue: s.formatNetworkValue(newField.Interface(), fieldName),
			})
		}
	}

	return changes
}

// buildNetworkCreationAudit 构建网络配置新增审计内容
func (s *ClusterResourceSync) buildNetworkCreationAudit(clusterName string, networkInfo interface{}) string {
	cniPlugin := s.getNetworkStringField(networkInfo, "CNIPlugin")
	if cniPlugin == "" {
		cniPlugin = "unknown"
	}

	proxyMode := s.getNetworkStringField(networkInfo, "ProxyMode")
	if proxyMode == "" {
		proxyMode = "iptables"
	}

	ingressController := s.getNetworkStringField(networkInfo, "IngressController")
	if ingressController == "" {
		ingressController = "unknown"
	}

	return fmt.Sprintf("集群[%s]网络配置初始化: CNI=%s, Proxy=%s, Ingress=%s",
		clusterName, cniPlugin, proxyMode, ingressController)
}

// buildNetworkUpdateAudit 构建网络配置更新审计内容
func (s *ClusterResourceSync) buildNetworkUpdateAudit(clusterName string, changes []FieldChange) string {
	if len(changes) == 0 {
		return fmt.Sprintf("集群[%s]网络配置无变更", clusterName)
	}

	// 提取关键变更
	var keyChanges []string
	for _, change := range changes {
		// 只记录重要的变更
		if isImportantNetworkField(change.Field) {
			keyChanges = append(keyChanges, fmt.Sprintf("%s: %v→%v",
				change.Field, change.OldValue, change.NewValue))
		}
	}

	if len(keyChanges) > 0 {
		return fmt.Sprintf("集群[%s]网络配置变更: %s", clusterName, strings.Join(keyChanges, ", "))
	}

	return fmt.Sprintf("集群[%s]网络配置变更: %d项", clusterName, len(changes))
}

// isImportantNetworkField 判断是否为重要的网络配置字段
func isImportantNetworkField(fieldName string) bool {
	importantFields := []string{
		"CNI插件", "CNI版本", "代理模式", "Ingress控制器",
		"IPv6启用", "双栈网络", "集群网络CIDR", "服务网络CIDR",
	}

	for _, field := range importantFields {
		if field == fieldName {
			return true
		}
	}

	return false
}

// formatNetworkValue 格式化网络配置字段值用于显示
func (s *ClusterResourceSync) formatNetworkValue(value interface{}, fieldName string) string {
	if value == nil {
		return "空"
	}

	switch v := value.(type) {
	case bool:
		if v {
			return "是"
		}
		return "否"
	case int64:
		if fieldName == "NodeCidrMaskSize" {
			return fmt.Sprintf("/%d", v)
		}
		if fieldName == "MtuSize" {
			return fmt.Sprintf("%d字节", v)
		}
		return fmt.Sprintf("%d", v)
	case string:
		if v == "" {
			return "空"
		}
		if v == "unknown" {
			return "未识别"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// detectClusterChanges 检测集群信息变更
func (s *ClusterResourceSync) detectClusterChanges(oldCluster, newCluster *model.OnecCluster) []FieldChange {
	var changes []FieldChange

	// 定义需要检测的字段映射（字段名 -> 中文名称）
	fieldsToCheck := map[string]string{
		"Version":          "Kubernetes版本",
		"GitCommit":        "Git提交号",
		"Platform":         "平台",
		"ClusterCreatedAt": "集群创建时间",
		"VersionBuildAt":   "版本构建时间",
		"ApiServerHost":    "API Server地址",
	}

	oldVal := reflect.ValueOf(oldCluster).Elem()
	newVal := reflect.ValueOf(newCluster).Elem()

	for fieldName, fieldDesc := range fieldsToCheck {
		oldField := oldVal.FieldByName(fieldName)
		newField := newVal.FieldByName(fieldName)

		if !oldField.IsValid() || !newField.IsValid() {
			continue
		}

		// 比较字段值
		if !s.compareFieldValues(oldField.Interface(), newField.Interface()) {
			changes = append(changes, FieldChange{
				Field:    fieldDesc,
				OldValue: s.formatFieldValue(oldField.Interface()),
				NewValue: s.formatFieldValue(newField.Interface()),
			})
		}
	}

	return changes
}

// compareFieldValues 比较字段值
func (s *ClusterResourceSync) compareFieldValues(old, new interface{}) bool {
	// 处理时间类型的比较
	if oldTime, ok := old.(time.Time); ok {
		if newTime, ok := new.(time.Time); ok {
			return oldTime.Equal(newTime)
		}
	}

	if oldFloat, ok := old.(float64); ok {
		if newFloat, ok := new.(float64); ok {
			return s.roundToDecimal(oldFloat, 2) == s.roundToDecimal(newFloat, 2)
		}
	}

	// 处理 float32 类型的比较
	if oldFloat, ok := old.(float32); ok {
		if newFloat, ok := new.(float32); ok {
			return s.roundToDecimal(float64(oldFloat), 2) == s.roundToDecimal(float64(newFloat), 2)
		}
	}

	// 其他类型直接比较
	return reflect.DeepEqual(old, new)
}

// roundToDecimal 四舍五入到指定小数位数（与数据库 DECIMAL 精度保持一致）
func (s *ClusterResourceSync) roundToDecimal(value float64, decimals int) float64 {
	multiplier := math.Pow(10, float64(decimals))
	return math.Round(value*multiplier) / multiplier
}

// formatFieldValue 格式化字段值用于显示
func (s *ClusterResourceSync) formatFieldValue(value interface{}) string {
	if value == nil {
		return "空"
	}

	switch v := value.(type) {
	case time.Time:
		if v.IsZero() {
			return "未设置"
		}
		return v.Format("2006-01-02 15:04:05")
	case bool:
		if v {
			return "是"
		}
		return "否"
	case int64:
		return fmt.Sprintf("%d", v)
	case string:
		if v == "" {
			return "空"
		}
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildChangeDescription 构建变更描述
func (s *ClusterResourceSync) buildChangeDescription(changes []FieldChange) string {
	if len(changes) == 0 {
		return "无变更"
	}

	var descriptions []string
	for _, change := range changes {
		desc := fmt.Sprintf("%s: [%v] -> [%v]",
			change.Field,
			change.OldValue,
			change.NewValue)
		descriptions = append(descriptions, desc)
	}

	return strings.Join(descriptions, "; ")
}

// calculateClusterResourceStats 计算集群资源统计（专用于资源同步）
func (s *ClusterResourceSync) calculateClusterResourceStats(ctx context.Context, clusterUuid string) (*clusterResourceStats, error) {
	// 获取集群版本信息
	clusterClient, err := s.K8sManager.GetCluster(ctx, clusterUuid)
	if err != nil {
		return nil, fmt.Errorf("获取集群客户端失败: %v", err)
	}

	node, err := clusterClient.Node().GetClusterResourceTotal()
	if err != nil {
		s.Logger.WithContext(ctx).Error("获取集群资源信息失败: %v", err)
		return nil, fmt.Errorf("获取集群资源信息失败: %v", err)
	}

	stats := &clusterResourceStats{
		TotalNodes:  int64(node.TotalNodes),
		TotalCpu:    node.TotalCpu,
		TotalMemory: node.TotalMemory,
		TotalPods:   node.TotalPods,
	}

	return stats, nil
}

// detectResourceChanges 检测资源信息变更
func (s *ClusterResourceSync) detectResourceChanges(oldResource, newResource *model.OnecClusterResource) []FieldChange {
	var changes []FieldChange

	// 定义需要检测的字段映射（不包括storage_capacity）
	fieldsToCheck := map[string]string{
		"CpuPhysicalCapacity":  "CPU容量",
		"MemPhysicalCapacity":  "内存容量",
		"PodsPhysicalCapacity": "Pod容量",
	}

	oldVal := reflect.ValueOf(oldResource).Elem()
	newVal := reflect.ValueOf(newResource).Elem()

	for fieldName, fieldDesc := range fieldsToCheck {
		oldField := oldVal.FieldByName(fieldName)
		newField := newVal.FieldByName(fieldName)

		if !oldField.IsValid() || !newField.IsValid() {
			continue
		}

		if !s.compareFieldValues(oldField.Interface(), newField.Interface()) {
			changes = append(changes, FieldChange{
				Field:    fieldDesc,
				OldValue: s.formatResourceValue(oldField.Interface(), fieldName),
				NewValue: s.formatResourceValue(newField.Interface(), fieldName),
			})
		}
	}

	return changes
}

// formatResourceValue 格式化资源字段值用于显示
func (s *ClusterResourceSync) formatResourceValue(value interface{}, fieldName string) string {
	if value == nil {
		return "0"
	}

	switch v := value.(type) {
	case int64:
		switch fieldName {
		case "CpuPhysicalCapacity":
			return fmt.Sprintf("%d核", v)
		case "MemPhysicalCapacity":
			return fmt.Sprintf("%dKi", v)
		case "PodsPhysicalCapacity":
			return fmt.Sprintf("%d个", v)
		default:
			return fmt.Sprintf("%d", v)
		}
	case float64:
		switch fieldName {
		case "CpuPhysicalCapacity":
			return fmt.Sprintf("%.0f核", v)
		case "MemPhysicalCapacity":
			return fmt.Sprintf("%.0fKi", v)
		case "PodsPhysicalCapacity":
			return fmt.Sprintf("%.0f个", v)
		default:
			return fmt.Sprintf("%.0f", v)
		}
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildNodeSyncSummaryAudit 构建节点同步汇总审计内容
func (s *ClusterResourceSync) buildNodeSyncSummaryAudit(clusterName string, addedCount, updatedCount, deletedCount, failedCount int,
	addedNodes, updatedNodes, deletedNodes, failedNodes []string, duration time.Duration) string {

	var parts []string

	// 新增节点
	if addedCount > 0 {
		if addedCount <= 5 {
			parts = append(parts, fmt.Sprintf("新增(%d): %s", addedCount, strings.Join(addedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("新增(%d): %s 等", addedCount, strings.Join(addedNodes[:5], ", ")))
		}
	}

	// 更新节点
	if updatedCount > 0 {
		if updatedCount <= 5 {
			parts = append(parts, fmt.Sprintf("更新(%d): %s", updatedCount, strings.Join(updatedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("更新(%d): %s 等", updatedCount, strings.Join(updatedNodes[:5], ", ")))
		}
	}

	// 删除节点
	if deletedCount > 0 {
		if deletedCount <= 5 {
			parts = append(parts, fmt.Sprintf("删除(%d): %s", deletedCount, strings.Join(deletedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("删除(%d): %s 等", deletedCount, strings.Join(deletedNodes[:5], ", ")))
		}
	}

	// 失败节点
	if failedCount > 0 {
		if failedCount <= 3 {
			parts = append(parts, fmt.Sprintf("失败(%d): %s", failedCount, strings.Join(failedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("失败(%d): %s 等", failedCount, strings.Join(failedNodes[:3], ", ")))
		}
	}

	result := fmt.Sprintf("集群[%s]节点同步完成, 耗时=%v", clusterName, duration)
	if len(parts) > 0 {
		result = fmt.Sprintf("%s: %s", result, strings.Join(parts, "; "))
	}

	return result
}

// buildClusterNodeModel 构建集群节点数据库模型
func (s *ClusterResourceSync) buildClusterNodeModel(clusterUuid string, nodeInfo *types.NodeInfo, operator string) *model.OnecClusterNode {
	memoryGiB := s.roundToDecimal(float64(nodeInfo.Memory)/1024/1024/1024, 2)
	cpu := s.roundToDecimal(nodeInfo.Cpu, 2)

	return &model.OnecClusterNode{
		ClusterUuid:     clusterUuid,
		NodeUuid:        nodeInfo.NodeUuid,
		Name:            nodeInfo.NodeName,
		Hostname:        nodeInfo.HostName,
		Roles:           nodeInfo.Roles,
		OsImage:         nodeInfo.OsImage,
		NodeIp:          nodeInfo.NodeIp,
		KernelVersion:   nodeInfo.KernelVersion,
		OperatingSystem: nodeInfo.OperatingSystem,
		Architecture:    nodeInfo.Architecture,
		Cpu:             cpu,
		Memory:          memoryGiB,
		Pods:            nodeInfo.Pods,
		IsGpu:           nodeInfo.IsGpu,
		Runtime:         nodeInfo.Runtime,
		JoinAt:          time.Unix(nodeInfo.JoinAt, 0),
		Unschedulable:   nodeInfo.Unschedulable,
		KubeletVersion:  nodeInfo.KubeletVersion,
		Status:          nodeInfo.Status,
		PodCidr:         nodeInfo.PodCidr,
		PodCidrs:        nodeInfo.PodCidrs,
		CreatedBy:       operator,
		UpdatedBy:       operator,
		IsDeleted:       0,
	}
}

// updateClusterNodeModel 更新集群节点模型
func (s *ClusterResourceSync) updateClusterNodeModel(node *model.OnecClusterNode, nodeInfo *types.NodeInfo, operator string) {
	memoryGiB := s.roundToDecimal(float64(nodeInfo.Memory)/1024/1024/1024, 2)
	cpu := s.roundToDecimal(nodeInfo.Cpu, 2)

	node.Name = nodeInfo.NodeName
	node.Hostname = nodeInfo.HostName
	node.Roles = nodeInfo.Roles
	node.OsImage = nodeInfo.OsImage
	node.NodeIp = nodeInfo.NodeIp
	node.KernelVersion = nodeInfo.KernelVersion
	node.OperatingSystem = nodeInfo.OperatingSystem
	node.Architecture = nodeInfo.Architecture
	node.Cpu = cpu
	node.Memory = memoryGiB
	node.Pods = nodeInfo.Pods
	node.IsGpu = nodeInfo.IsGpu
	node.Runtime = nodeInfo.Runtime
	node.JoinAt = time.Unix(nodeInfo.JoinAt, 0)
	node.Unschedulable = nodeInfo.Unschedulable
	node.KubeletVersion = nodeInfo.KubeletVersion
	node.Status = nodeInfo.Status
	node.PodCidr = nodeInfo.PodCidr
	node.PodCidrs = nodeInfo.PodCidrs
	node.UpdatedBy = operator
}

// detectNodeChanges 检测节点信息变更
func (s *ClusterResourceSync) detectNodeChanges(oldNode, newNode *model.OnecClusterNode) []FieldChange {
	var changes []FieldChange

	fieldsToCheck := map[string]string{
		"Name":            "节点名称",
		"Hostname":        "主机名",
		"Roles":           "节点角色",
		"OsImage":         "操作系统镜像",
		"NodeIp":          "节点IP",
		"KernelVersion":   "内核版本",
		"OperatingSystem": "操作系统",
		"Architecture":    "架构",
		"Cpu":             "CPU",
		"Memory":          "内存",
		"Pods":            "最大Pod数",
		"IsGpu":           "GPU节点",
		"Runtime":         "容器运行时",
		"Unschedulable":   "调度状态",
		"KubeletVersion":  "Kubelet版本",
		"Status":          "节点状态",
		"PodCidr":         "Pod CIDR",
		"PodCidrs":        "Pod CIDRs",
	}

	oldVal := reflect.ValueOf(oldNode).Elem()
	newVal := reflect.ValueOf(newNode).Elem()

	for fieldName, fieldDesc := range fieldsToCheck {
		oldField := oldVal.FieldByName(fieldName)
		newField := newVal.FieldByName(fieldName)

		if !oldField.IsValid() || !newField.IsValid() {
			continue
		}

		if !s.compareFieldValues(oldField.Interface(), newField.Interface()) {
			changes = append(changes, FieldChange{
				Field:    fieldDesc,
				OldValue: s.formatNodeFieldValue(oldField.Interface(), fieldName),
				NewValue: s.formatNodeFieldValue(newField.Interface(), fieldName),
			})
		}
	}

	return changes
}

// formatNodeFieldValue 格式化节点字段值用于显示
func (s *ClusterResourceSync) formatNodeFieldValue(value interface{}, fieldName string) string {
	if value == nil {
		return "空"
	}

	switch v := value.(type) {
	case string:
		if v == "" {
			return "空"
		}
		return v
	case float64:
		switch fieldName {
		case "Cpu":
			return fmt.Sprintf("%.2f核", v)
		case "Memory":
			return fmt.Sprintf("%.2fGi", v)
		default:
			return fmt.Sprintf("%.2f", v)
		}
	case int64:
		switch fieldName {
		case "Pods":
			return fmt.Sprintf("%d个", v)
		case "IsGpu":
			if v == 1 {
				return "是"
			}
			return "否"
		case "Unschedulable":
			if v == 1 {
				return "不可调度"
			}
			return "可调度"
		default:
			return fmt.Sprintf("%d", v)
		}
	case time.Time:
		if v.IsZero() {
			return "未设置"
		}
		return v.Format("2006-01-02 15:04:05")
	default:
		return fmt.Sprintf("%v", v)
	}
}

// buildNodeSyncAuditContent 构建节点同步审计内容（保留兼容）
func (s *ClusterResourceSync) buildNodeSyncAuditContent(clusterName string, addedCount, updatedCount, deletedCount int, addedNodes, updatedNodes, deletedNodes []string) string {
	var parts []string

	if addedCount > 0 {
		if addedCount <= 3 {
			parts = append(parts, fmt.Sprintf("新增节点: %s", strings.Join(addedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("新增节点: %d个", addedCount))
		}
	}

	if updatedCount > 0 {
		if updatedCount <= 3 {
			parts = append(parts, fmt.Sprintf("更新节点: %s", strings.Join(updatedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("更新节点: %d个", updatedCount))
		}
	}

	if deletedCount > 0 {
		if deletedCount <= 3 {
			parts = append(parts, fmt.Sprintf("删除节点: %s", strings.Join(deletedNodes, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("删除节点: %d个", deletedCount))
		}
	}

	if len(parts) == 0 {
		return fmt.Sprintf("集群[%s]节点无变化", clusterName)
	}

	return fmt.Sprintf("集群[%s]节点同步: %s", clusterName, strings.Join(parts, "; "))
}
