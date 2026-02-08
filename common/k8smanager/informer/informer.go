package informer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// Manager Informer 管理器
type Manager struct {
	// 基础客户端
	kubeClient    kubernetes.Interface
	dynamicClient dynamic.Interface

	// Informer 工厂
	informerFactory        informers.SharedInformerFactory
	dynamicInformerFactory dynamicinformer.DynamicSharedInformerFactory

	// 资源配置
	resources []ResourceInfo

	// Informer 映射表
	informers map[ResourceType]cache.SharedIndexInformer

	// 停止信号
	stopCh chan struct{}

	// 停止标志（确保只停止一次）
	stopOnce sync.Once

	// 同步状态
	synced bool
	mu     sync.RWMutex

	// 日志
	logger logx.Logger
}

// NewManager 创建 Informer 管理器
func NewManager(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	resyncPeriod time.Duration,
) *Manager {
	if resyncPeriod == 0 {
		resyncPeriod = 10 * time.Minute
	}

	return &Manager{
		kubeClient:             kubeClient,
		dynamicClient:          dynamicClient,
		informerFactory:        informers.NewSharedInformerFactory(kubeClient, resyncPeriod),
		dynamicInformerFactory: dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, resyncPeriod),
		resources:              GetEnabledResources(),
		informers:              make(map[ResourceType]cache.SharedIndexInformer),
		stopCh:                 make(chan struct{}),
		logger:                 logx.WithContext(context.Background()),
	}
}

// WithResources 设置要监控的资源
func (m *Manager) WithResources(resources []ResourceInfo) *Manager {
	m.resources = resources
	return m
}

// WithLogger 设置日志器
func (m *Manager) WithLogger(logger logx.Logger) *Manager {
	m.logger = logger
	return m
}

// RegisterInformers 注册所有 Informer
func (m *Manager) RegisterInformers() error {
	m.logger.Infof("开始注册 Informer，共 %d 个资源类型", len(m.resources))

	for _, resource := range m.resources {
		if !resource.Enabled {
			m.logger.Debugf("跳过未启用的资源: %s", resource.Type)
			continue
		}

		if err := m.registerInformer(resource); err != nil {
			m.logger.Errorf("注册 %s Informer 失败: %v", resource.Type, err)
			// 继续注册其他 Informer，不中断
			continue
		}

		m.logger.Infof("成功注册 %s Informer", resource.Type)
	}

	m.logger.Infof("Informer 注册完成，成功注册 %d 个", len(m.informers))
	return nil
}

// registerInformer 注册单个 Informer
func (m *Manager) registerInformer(resource ResourceInfo) error {
	var informer cache.SharedIndexInformer

	// 根据资源类型选择合适的 Informer
	switch resource.Type {
	// 核心资源 v1
	case ResourceNamespace:
		informer = m.informerFactory.Core().V1().Namespaces().Informer()
	case ResourcePod:
		informer = m.informerFactory.Core().V1().Pods().Informer()
	case ResourceService:
		informer = m.informerFactory.Core().V1().Services().Informer()
	case ResourceEndpoints:
		informer = m.informerFactory.Core().V1().Endpoints().Informer()
	case ResourceConfigMap:
		informer = m.informerFactory.Core().V1().ConfigMaps().Informer()
	case ResourceSecret:
		informer = m.informerFactory.Core().V1().Secrets().Informer()
	case ResourceNode:
		informer = m.informerFactory.Core().V1().Nodes().Informer()
	case ResourcePVC:
		informer = m.informerFactory.Core().V1().PersistentVolumeClaims().Informer()
	case ResourcePV:
		informer = m.informerFactory.Core().V1().PersistentVolumes().Informer()
	// 工作负载资源 apps/v1
	case ResourceDeployment:
		informer = m.informerFactory.Apps().V1().Deployments().Informer()
	case ResourceStatefulSet:
		informer = m.informerFactory.Apps().V1().StatefulSets().Informer()
	case ResourceDaemonSet:
		informer = m.informerFactory.Apps().V1().DaemonSets().Informer()
	case ResourceReplicaSet:
		informer = m.informerFactory.Apps().V1().ReplicaSets().Informer()
	// 批处理资源 batch/v1
	case ResourceJob:
		informer = m.informerFactory.Batch().V1().Jobs().Informer()
	case ResourceCronJob:
		informer = m.informerFactory.Batch().V1().CronJobs().Informer()

	// 网络资源 networking.k8s.io/v1
	case ResourceIngress:
		informer = m.informerFactory.Networking().V1().Ingresses().Informer()
	case ResourceNetworkPolicy:
		informer = m.informerFactory.Networking().V1().NetworkPolicies().Informer()

	// 存储资源 storage.k8s.io/v1
	case ResourceStorageClass:
		informer = m.informerFactory.Storage().V1().StorageClasses().Informer()

	// RBAC资源
	case ResourceServiceAccount:
		informer = m.informerFactory.Core().V1().ServiceAccounts().Informer()
	case ResourceRole:
		informer = m.informerFactory.Rbac().V1().Roles().Informer()
	case ResourceRoleBinding:
		informer = m.informerFactory.Rbac().V1().RoleBindings().Informer()
	case ResourceClusterRole:
		informer = m.informerFactory.Rbac().V1().ClusterRoles().Informer()
	case ResourceClusterRoleBinding:
		informer = m.informerFactory.Rbac().V1().ClusterRoleBindings().Informer()

	default:
		// 使用动态 Informer 作为后备方案
		gvr := schema.GroupVersionResource{
			Group:    resource.GroupVersion.Group,
			Version:  resource.GroupVersion.Version,
			Resource: resource.Resource,
		}
		informer = m.dynamicInformerFactory.ForResource(gvr).Informer()
	}

	// 添加事件处理器
	_, err := informer.AddEventHandler(m.createEventHandler(resource.Type))
	if err != nil {
		return err
	}

	// 保存 Informer
	m.informers[resource.Type] = informer

	return nil
}

// createEventHandler 创建事件处理器
func (m *Manager) createEventHandler(resourceType ResourceType) cache.ResourceEventHandlerFuncs {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			m.logger.Debugf("[%s] 资源添加事件", resourceType)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			m.logger.Debugf("[%s] 资源更新事件", resourceType)
		},
		DeleteFunc: func(obj interface{}) {
			// 处理 DeletedFinalStateUnknown
			if deletedFinal, ok := obj.(cache.DeletedFinalStateUnknown); ok {
				obj = deletedFinal.Obj
			}
			m.logger.Debugf("[%s] 资源删除事件", resourceType)
		},
	}
}

// Start 启动所有 Informer
func (m *Manager) Start() error {
	m.logger.Info("启动 Informer 管理器...")

	// 启动标准 Informer Factory
	m.informerFactory.Start(m.stopCh)

	// 启动动态 Informer Factory
	m.dynamicInformerFactory.Start(m.stopCh)

	m.logger.Info("Informer 已启动，等待缓存同步...")

	// 等待缓存同步
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := m.WaitForCacheSync(ctx); err != nil {
		return fmt.Errorf("等待缓存同步失败: %w", err)
	}

	m.logger.Info("Informer 管理器启动完成")
	return nil
}

// WaitForCacheSync 等待缓存同步
func (m *Manager) WaitForCacheSync(ctx context.Context) error {
	// 收集所有需要同步的 Informer
	var syncFuncs []cache.InformerSynced
	for resourceType, informer := range m.informers {
		m.logger.Debugf("等待 %s Informer 同步...", resourceType)
		syncFuncs = append(syncFuncs, informer.HasSynced)
	}

	// 等待所有 Informer 同步
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("等待缓存同步超时")
		case <-ticker.C:
			allSynced := true
			for _, syncFunc := range syncFuncs {
				if !syncFunc() {
					allSynced = false
					break
				}
			}

			if allSynced {
				m.mu.Lock()
				m.synced = true
				m.mu.Unlock()
				m.logger.Info("所有 Informer 缓存同步完成")
				return nil
			}
		}
	}
}

// Stop 停止所有 Informer
func (m *Manager) Stop() {
	m.stopOnce.Do(func() {
		m.logger.Info("停止 Informer 管理器...")
		close(m.stopCh)

		m.mu.Lock()
		m.synced = false
		m.mu.Unlock()

		m.logger.Info("Informer 管理器已停止")
	})
}

// HasSynced 检查是否已同步
func (m *Manager) HasSynced() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.synced
}

// GetInformer 获取指定资源的 Informer
func (m *Manager) GetInformer(resourceType ResourceType) (cache.SharedIndexInformer, bool) {
	informer, exists := m.informers[resourceType]
	return informer, exists
}

// GetInformerFactory 获取标准 Informer Factory
func (m *Manager) GetInformerFactory() informers.SharedInformerFactory {
	return m.informerFactory
}

// GetDynamicInformerFactory 获取动态 Informer Factory
func (m *Manager) GetDynamicInformerFactory() dynamicinformer.DynamicSharedInformerFactory {
	return m.dynamicInformerFactory
}
