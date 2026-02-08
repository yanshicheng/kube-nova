package incremental

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

// ClusterWatcher 单个集群的资源监听器
// 通过 K8s SharedInformerFactory 监听集群中的资源变更事件，并将事件发送到 EventProcessor 处理
type ClusterWatcher struct {
	clusterUUID string
	client      kubernetes.Interface
	enqueuer    EventEnqueuer
	factory     informers.SharedInformerFactory

	mu       sync.Mutex
	running  int32
	stopped  int32
	stopCh   chan struct{}
	stopOnce sync.Once

	// 用于通知 watcher 已完全停止
	doneCh chan struct{}

	startTime      time.Time
	eventsReceived int64
	eventsDropped  int64

	// DELETE 事件统计
	deleteReceived int64
	deleteDropped  int64

	// 待处理事件追踪
	pendingCount int64 // 已入队但未处理完成的事件数量
}

// NewClusterWatcher 创建集群监听器
func NewClusterWatcher(clusterUUID string, client kubernetes.Interface, enqueuer EventEnqueuer) *ClusterWatcher {
	return &ClusterWatcher{
		clusterUUID: clusterUUID,
		client:      client,
		enqueuer:    enqueuer,
		doneCh:      make(chan struct{}),
	}
}

// Start 启动监听
// 此方法会阻塞直到收到停止信号或 context 被取消
func (w *ClusterWatcher) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		logx.Errorf("[ClusterWatcher] 集群 %s: 已在运行中，跳过启动", w.clusterUUID)
		return
	}

	w.mu.Lock()
	w.startTime = time.Now()
	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.stopOnce = sync.Once{}
	atomic.StoreInt64(&w.eventsReceived, 0)
	atomic.StoreInt64(&w.eventsDropped, 0)
	atomic.StoreInt64(&w.deleteReceived, 0)
	atomic.StoreInt64(&w.deleteDropped, 0)
	atomic.StoreInt64(&w.pendingCount, 0)
	atomic.StoreInt32(&w.stopped, 0)
	w.mu.Unlock()

	// 创建 SharedInformerFactory，resync 周期 30 秒
	w.factory = informers.NewSharedInformerFactory(w.client, 30*time.Second)

	// 注册所有 Informer
	w.registerNamespaceInformer()
	w.registerDeploymentInformer()
	w.registerStatefulSetInformer()
	w.registerDaemonSetInformer()
	w.registerCronJobInformer()
	w.registerResourceQuotaInformer()
	w.registerLimitRangeInformer()

	// 启动 Informer
	w.factory.Start(w.stopCh)

	logx.Infof("[ClusterWatcher] 集群 %s: 等待 Informer 缓存同步...", w.clusterUUID)
	synced := w.factory.WaitForCacheSync(w.stopCh)
	allSynced := true
	for informerType, ok := range synced {
		if !ok {
			allSynced = false
			logx.Errorf("[ClusterWatcher] 集群 %s: Informer %v 缓存同步失败", w.clusterUUID, informerType)
		}
	}

	if allSynced {
		logx.Infof("[ClusterWatcher] 集群 %s: 所有 Informer 缓存同步完成，开始监听", w.clusterUUID)
	} else {
		logx.Errorf("[ClusterWatcher] 集群 %s: 部分 Informer 缓存同步失败，继续监听", w.clusterUUID)
	}

	// 等待停止信号
	select {
	case <-ctx.Done():
		logx.Infof("[ClusterWatcher] 集群 %s: 收到 context 取消信号", w.clusterUUID)
	case <-w.stopCh:
		logx.Infof("[ClusterWatcher] 集群 %s: 收到停止信号", w.clusterUUID)
	}

	w.doStop()
}

// Stop 停止监听（幂等）
// 此方法只发送停止信号，不等待完成
func (w *ClusterWatcher) Stop() {
	w.stopOnce.Do(func() {
		atomic.StoreInt32(&w.stopped, 1)
		w.mu.Lock()
		if w.stopCh != nil {
			close(w.stopCh)
		}
		w.mu.Unlock()
	})
}

// WaitForStop 等待监听器完全停止
// 返回 true 表示正常停止，false 表示超时
func (w *ClusterWatcher) WaitForStop(timeout time.Duration) bool {
	select {
	case <-w.doneCh:
		return true
	case <-time.After(timeout):
		return false
	}
}

// WaitForPendingEvents 等待所有待处理事件完成
// 返回 true 表示所有事件已处理完成，false 表示超时
func (w *ClusterWatcher) WaitForPendingEvents(timeout time.Duration) bool {
	if atomic.LoadInt64(&w.pendingCount) == 0 {
		return true
	}

	deadline := time.Now().Add(timeout)
	checkInterval := 100 * time.Millisecond

	for time.Now().Before(deadline) {
		if atomic.LoadInt64(&w.pendingCount) == 0 {
			return true
		}
		time.Sleep(checkInterval)
	}

	remaining := atomic.LoadInt64(&w.pendingCount)
	if remaining > 0 {
		logx.Errorf("[ClusterWatcher] 集群 %s: 等待待处理事件超时，剩余 %d 个事件", w.clusterUUID, remaining)
		return false
	}
	return true
}

// DecrementPending 减少待处理事件计数
// 由 EventProcessor 在事件处理完成后调用
func (w *ClusterWatcher) DecrementPending() {
	atomic.AddInt64(&w.pendingCount, -1)
}

// GetPendingCount 获取当前待处理事件数量
func (w *ClusterWatcher) GetPendingCount() int64 {
	return atomic.LoadInt64(&w.pendingCount)
}

// doStop 执行实际的停止逻辑
func (w *ClusterWatcher) doStop() {
	if !atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		return
	}

	uptime := time.Since(w.startTime)
	pending := atomic.LoadInt64(&w.pendingCount)
	logx.Infof("[ClusterWatcher] 集群 %s: 监听已停止, 运行时长: %v, 接收事件: %d (DELETE: %d), 丢弃事件: %d (DELETE: %d), 待处理: %d",
		w.clusterUUID, uptime.Round(time.Second),
		atomic.LoadInt64(&w.eventsReceived),
		atomic.LoadInt64(&w.deleteReceived),
		atomic.LoadInt64(&w.eventsDropped),
		atomic.LoadInt64(&w.deleteDropped),
		pending)

	// 通知等待者监听器已停止
	w.mu.Lock()
	if w.doneCh != nil {
		close(w.doneCh)
	}
	w.mu.Unlock()
}

// IsRunning 返回监听器是否在运行
func (w *ClusterWatcher) IsRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// GetStats 获取监听器统计信息
func (w *ClusterWatcher) GetStats() *WatcherStats {
	w.mu.Lock()
	startTime := w.startTime
	w.mu.Unlock()

	return &WatcherStats{
		ClusterUUID:    w.clusterUUID,
		IsRunning:      w.IsRunning(),
		StartTime:      startTime,
		EventsReceived: atomic.LoadInt64(&w.eventsReceived),
		PendingEvents:  atomic.LoadInt64(&w.pendingCount),
	}
}

// enqueueEvent 将事件入队
// DELETE 事件即使在 watcher 停止后也要尽力发送
func (w *ClusterWatcher) enqueueEvent(event *ResourceEvent) {
	isDelete := event.IsDelete()

	// 统计 DELETE 事件
	if isDelete {
		atomic.AddInt64(&w.deleteReceived, 1)
	}

	// 检查 enqueuer 是否可用
	if w.enqueuer == nil {
		if isDelete {
			atomic.AddInt64(&w.deleteDropped, 1)
			logx.Errorf("[ClusterWatcher] 集群 %s: Enqueuer 不可用，DELETE 事件被丢弃: %s/%s/%s",
				w.clusterUUID, event.ResourceType, event.Namespace, event.Name)
		} else {
			atomic.AddInt64(&w.eventsDropped, 1)
			logx.Debugf("[ClusterWatcher] 集群 %s: Enqueuer 不可用，丢弃事件: %s/%s/%s",
				w.clusterUUID, event.Type, event.ResourceType, event.Name)
		}
		return
	}

	// 检查 watcher 是否已停止
	if atomic.LoadInt32(&w.stopped) == 1 {
		// DELETE 事件即使 watcher 停止也要尝试发送
		if isDelete {
			logx.Infof("[ClusterWatcher] 集群 %s: Watcher 已停止，但仍发送 DELETE 事件: %s/%s/%s",
				w.clusterUUID, event.ResourceType, event.Namespace, event.Name)
		} else {
			atomic.AddInt64(&w.eventsDropped, 1)
			logx.Debugf("[ClusterWatcher] 集群 %s: Watcher 已停止，丢弃非 DELETE 事件: %s/%s/%s",
				w.clusterUUID, event.Type, event.ResourceType, event.Name)
			return
		}
	}

	atomic.AddInt64(&w.eventsReceived, 1)

	// 记录事件接收日志（DELETE 事件使用 Info 级别，其他使用 Debug）
	if isDelete {
		logx.Infof("[ClusterWatcher] 集群 %s: 接收 DELETE 事件: Resource=%s, Namespace=%s, Name=%s",
			w.clusterUUID, event.ResourceType, event.Namespace, event.Name)
	} else {
		logx.Debugf("[ClusterWatcher] 集群 %s: 接收事件: Type=%s, Resource=%s, Namespace=%s, Name=%s",
			w.clusterUUID, event.Type, event.ResourceType, event.Namespace, event.Name)
	}

	// 增加待处理计数
	atomic.AddInt64(&w.pendingCount, 1)

	// 入队事件
	if !w.enqueuer.EnqueueEvent(event) {
		// 入队失败，减少待处理计数
		atomic.AddInt64(&w.pendingCount, -1)
		if isDelete {
			atomic.AddInt64(&w.deleteDropped, 1)
		} else {
			atomic.AddInt64(&w.eventsDropped, 1)
		}
	}
}

// ==================== Namespace Informer ====================

func (w *ClusterWatcher) registerNamespaceInformer() {
	informer := w.factory.Core().V1().Namespaces().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				logx.Errorf("[ClusterWatcher] 集群 %s: Namespace AddFunc 类型断言失败", w.clusterUUID)
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "namespace",
				Namespace:    "",
				Name:         ns.Name,
				NewObject:    ns,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldNs, ok := oldObj.(*corev1.Namespace)
			if !ok {
				return
			}
			newNs, ok := newObj.(*corev1.Namespace)
			if !ok {
				return
			}
			// 跳过无变化的事件
			if oldNs.ResourceVersion == newNs.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "namespace",
				Namespace:    "",
				Name:         newNs.Name,
				OldObject:    oldNs,
				NewObject:    newNs,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var ns *corev1.Namespace
			switch t := obj.(type) {
			case *corev1.Namespace:
				ns = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				ns, ok = t.Obj.(*corev1.Namespace)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: Namespace DeleteFunc tombstone 类型断言失败: %T",
						w.clusterUUID, t.Obj)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 Namespace 删除事件: %s",
					w.clusterUUID, ns.Name)
			default:
				logx.Errorf("[ClusterWatcher] 集群 %s: Namespace DeleteFunc 未知类型: %T",
					w.clusterUUID, obj)
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 Namespace DELETE 事件: %s", w.clusterUUID, ns.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "namespace",
				Namespace:    "",
				Name:         ns.Name,
				OldObject:    ns,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 Namespace Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== Deployment Informer ====================

func (w *ClusterWatcher) registerDeploymentInformer() {
	informer := w.factory.Apps().V1().Deployments().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			deploy, ok := obj.(*appsv1.Deployment)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "deployment",
				Namespace:    deploy.Namespace,
				Name:         deploy.Name,
				NewObject:    deploy,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDeploy, ok := oldObj.(*appsv1.Deployment)
			if !ok {
				return
			}
			newDeploy, ok := newObj.(*appsv1.Deployment)
			if !ok {
				return
			}
			if oldDeploy.ResourceVersion == newDeploy.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "deployment",
				Namespace:    newDeploy.Namespace,
				Name:         newDeploy.Name,
				OldObject:    oldDeploy,
				NewObject:    newDeploy,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var deploy *appsv1.Deployment
			switch t := obj.(type) {
			case *appsv1.Deployment:
				deploy = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				deploy, ok = t.Obj.(*appsv1.Deployment)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: Deployment DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 Deployment 删除事件: %s/%s",
					w.clusterUUID, deploy.Namespace, deploy.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 Deployment DELETE 事件: %s/%s",
				w.clusterUUID, deploy.Namespace, deploy.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "deployment",
				Namespace:    deploy.Namespace,
				Name:         deploy.Name,
				OldObject:    deploy,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 Deployment Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== StatefulSet Informer ====================

func (w *ClusterWatcher) registerStatefulSetInformer() {
	informer := w.factory.Apps().V1().StatefulSets().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			sts, ok := obj.(*appsv1.StatefulSet)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "statefulset",
				Namespace:    sts.Namespace,
				Name:         sts.Name,
				NewObject:    sts,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldSts, ok := oldObj.(*appsv1.StatefulSet)
			if !ok {
				return
			}
			newSts, ok := newObj.(*appsv1.StatefulSet)
			if !ok {
				return
			}
			if oldSts.ResourceVersion == newSts.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "statefulset",
				Namespace:    newSts.Namespace,
				Name:         newSts.Name,
				OldObject:    oldSts,
				NewObject:    newSts,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var sts *appsv1.StatefulSet
			switch t := obj.(type) {
			case *appsv1.StatefulSet:
				sts = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				sts, ok = t.Obj.(*appsv1.StatefulSet)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: StatefulSet DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 StatefulSet 删除事件: %s/%s",
					w.clusterUUID, sts.Namespace, sts.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 StatefulSet DELETE 事件: %s/%s",
				w.clusterUUID, sts.Namespace, sts.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "statefulset",
				Namespace:    sts.Namespace,
				Name:         sts.Name,
				OldObject:    sts,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 StatefulSet Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== DaemonSet Informer ====================

func (w *ClusterWatcher) registerDaemonSetInformer() {
	informer := w.factory.Apps().V1().DaemonSets().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ds, ok := obj.(*appsv1.DaemonSet)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "daemonset",
				Namespace:    ds.Namespace,
				Name:         ds.Name,
				NewObject:    ds,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldDs, ok := oldObj.(*appsv1.DaemonSet)
			if !ok {
				return
			}
			newDs, ok := newObj.(*appsv1.DaemonSet)
			if !ok {
				return
			}
			if oldDs.ResourceVersion == newDs.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "daemonset",
				Namespace:    newDs.Namespace,
				Name:         newDs.Name,
				OldObject:    oldDs,
				NewObject:    newDs,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var ds *appsv1.DaemonSet
			switch t := obj.(type) {
			case *appsv1.DaemonSet:
				ds = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				ds, ok = t.Obj.(*appsv1.DaemonSet)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: DaemonSet DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 DaemonSet 删除事件: %s/%s",
					w.clusterUUID, ds.Namespace, ds.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 DaemonSet DELETE 事件: %s/%s",
				w.clusterUUID, ds.Namespace, ds.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "daemonset",
				Namespace:    ds.Namespace,
				Name:         ds.Name,
				OldObject:    ds,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 DaemonSet Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== CronJob Informer ====================

func (w *ClusterWatcher) registerCronJobInformer() {
	informer := w.factory.Batch().V1().CronJobs().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			cj, ok := obj.(*batchv1.CronJob)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "cronjob",
				Namespace:    cj.Namespace,
				Name:         cj.Name,
				NewObject:    cj,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldCj, ok := oldObj.(*batchv1.CronJob)
			if !ok {
				return
			}
			newCj, ok := newObj.(*batchv1.CronJob)
			if !ok {
				return
			}
			if oldCj.ResourceVersion == newCj.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "cronjob",
				Namespace:    newCj.Namespace,
				Name:         newCj.Name,
				OldObject:    oldCj,
				NewObject:    newCj,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var cj *batchv1.CronJob
			switch t := obj.(type) {
			case *batchv1.CronJob:
				cj = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				cj, ok = t.Obj.(*batchv1.CronJob)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: CronJob DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 CronJob 删除事件: %s/%s",
					w.clusterUUID, cj.Namespace, cj.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 CronJob DELETE 事件: %s/%s",
				w.clusterUUID, cj.Namespace, cj.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "cronjob",
				Namespace:    cj.Namespace,
				Name:         cj.Name,
				OldObject:    cj,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 CronJob Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== ResourceQuota Informer ====================

func (w *ClusterWatcher) registerResourceQuotaInformer() {
	informer := w.factory.Core().V1().ResourceQuotas().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			rq, ok := obj.(*corev1.ResourceQuota)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "resourcequota",
				Namespace:    rq.Namespace,
				Name:         rq.Name,
				NewObject:    rq,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldRq, ok := oldObj.(*corev1.ResourceQuota)
			if !ok {
				return
			}
			newRq, ok := newObj.(*corev1.ResourceQuota)
			if !ok {
				return
			}
			if oldRq.ResourceVersion == newRq.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "resourcequota",
				Namespace:    newRq.Namespace,
				Name:         newRq.Name,
				OldObject:    oldRq,
				NewObject:    newRq,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var rq *corev1.ResourceQuota
			switch t := obj.(type) {
			case *corev1.ResourceQuota:
				rq = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				rq, ok = t.Obj.(*corev1.ResourceQuota)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: ResourceQuota DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 ResourceQuota 删除事件: %s/%s",
					w.clusterUUID, rq.Namespace, rq.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 ResourceQuota DELETE 事件: %s/%s",
				w.clusterUUID, rq.Namespace, rq.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "resourcequota",
				Namespace:    rq.Namespace,
				Name:         rq.Name,
				OldObject:    rq,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 ResourceQuota Informer 失败: %v", w.clusterUUID, err)
	}
}

// ==================== LimitRange Informer ====================

func (w *ClusterWatcher) registerLimitRangeInformer() {
	informer := w.factory.Core().V1().LimitRanges().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			lr, ok := obj.(*corev1.LimitRange)
			if !ok {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventAdd,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "limitrange",
				Namespace:    lr.Namespace,
				Name:         lr.Name,
				NewObject:    lr,
				Timestamp:    time.Now(),
			})
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldLr, ok := oldObj.(*corev1.LimitRange)
			if !ok {
				return
			}
			newLr, ok := newObj.(*corev1.LimitRange)
			if !ok {
				return
			}
			if oldLr.ResourceVersion == newLr.ResourceVersion {
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventUpdate,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "limitrange",
				Namespace:    newLr.Namespace,
				Name:         newLr.Name,
				OldObject:    oldLr,
				NewObject:    newLr,
				Timestamp:    time.Now(),
			})
		},
		DeleteFunc: func(obj interface{}) {
			var lr *corev1.LimitRange
			switch t := obj.(type) {
			case *corev1.LimitRange:
				lr = t
			case cache.DeletedFinalStateUnknown:
				var ok bool
				lr, ok = t.Obj.(*corev1.LimitRange)
				if !ok {
					logx.Errorf("[ClusterWatcher] 集群 %s: LimitRange DeleteFunc tombstone 类型断言失败",
						w.clusterUUID)
					return
				}
				logx.Infof("[ClusterWatcher] 集群 %s: 从 tombstone 恢复 LimitRange 删除事件: %s/%s",
					w.clusterUUID, lr.Namespace, lr.Name)
			default:
				return
			}

			logx.Infof("[ClusterWatcher] 集群 %s: 捕获 LimitRange DELETE 事件: %s/%s",
				w.clusterUUID, lr.Namespace, lr.Name)

			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "limitrange",
				Namespace:    lr.Namespace,
				Name:         lr.Name,
				OldObject:    lr,
				Timestamp:    time.Now(),
				Priority:     PriorityHigh,
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 LimitRange Informer 失败: %v", w.clusterUUID, err)
	}
}
