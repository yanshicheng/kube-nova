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
type ClusterWatcher struct {
	clusterUUID string
	client      kubernetes.Interface
	processor   *EventProcessor
	factory     informers.SharedInformerFactory

	mu       sync.Mutex
	running  int32
	stopped  int32
	stopCh   chan struct{}
	stopOnce sync.Once

	startTime      time.Time
	eventsReceived int64
}

// NewClusterWatcher 创建集群监听器
func NewClusterWatcher(clusterUUID string, client kubernetes.Interface, processor *EventProcessor) *ClusterWatcher {
	return &ClusterWatcher{
		clusterUUID: clusterUUID,
		client:      client,
		processor:   processor,
	}
}

// Start 启动监听
func (w *ClusterWatcher) Start(ctx context.Context) {
	if !atomic.CompareAndSwapInt32(&w.running, 0, 1) {
		logx.Errorf("[ClusterWatcher] 集群 %s: 已在运行中，跳过启动", w.clusterUUID)
		return
	}

	w.mu.Lock()
	w.startTime = time.Now()
	w.stopCh = make(chan struct{})
	w.stopOnce = sync.Once{}
	atomic.StoreInt64(&w.eventsReceived, 0)
	atomic.StoreInt32(&w.stopped, 0)
	w.mu.Unlock()

	w.factory = informers.NewSharedInformerFactory(w.client, 30*time.Second)

	w.registerNamespaceInformer()
	w.registerDeploymentInformer()
	w.registerStatefulSetInformer()
	w.registerDaemonSetInformer()
	w.registerCronJobInformer()
	w.registerResourceQuotaInformer()
	w.registerLimitRangeInformer()

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

	select {
	case <-ctx.Done():
		logx.Infof("[ClusterWatcher] 集群 %s: 收到 context 取消信号", w.clusterUUID)
	case <-w.stopCh:
		logx.Infof("[ClusterWatcher] 集群 %s: 收到停止信号", w.clusterUUID)
	}

	w.doStop()
}

// Stop 停止监听（幂等）
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

func (w *ClusterWatcher) doStop() {
	if !atomic.CompareAndSwapInt32(&w.running, 1, 0) {
		return
	}

	uptime := time.Since(w.startTime)
	logx.Infof("[ClusterWatcher] 集群 %s: 监听已停止, 运行时长: %v, 接收事件数: %d",
		w.clusterUUID, uptime.Round(time.Second), atomic.LoadInt64(&w.eventsReceived))
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
	}
}

func (w *ClusterWatcher) enqueueEvent(event *ResourceEvent) {
	if atomic.LoadInt32(&w.stopped) == 1 {
		return
	}
	atomic.AddInt64(&w.eventsReceived, 1)
	w.processor.EnqueueEvent(event)
}

func (w *ClusterWatcher) registerNamespaceInformer() {
	informer := w.factory.Core().V1().Namespaces().Informer()

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "namespace",
				Namespace:    "",
				Name:         ns.Name,
				OldObject:    ns,
				Timestamp:    time.Now(),
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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "deployment",
				Namespace:    deploy.Namespace,
				Name:         deploy.Name,
				OldObject:    deploy,
				Timestamp:    time.Now(),
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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "statefulset",
				Namespace:    sts.Namespace,
				Name:         sts.Name,
				OldObject:    sts,
				Timestamp:    time.Now(),
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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "daemonset",
				Namespace:    ds.Namespace,
				Name:         ds.Name,
				OldObject:    ds,
				Timestamp:    time.Now(),
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 DaemonSet Informer 失败: %v", w.clusterUUID, err)
	}
}

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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "cronjob",
				Namespace:    cj.Namespace,
				Name:         cj.Name,
				OldObject:    cj,
				Timestamp:    time.Now(),
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 CronJob Informer 失败: %v", w.clusterUUID, err)
	}
}

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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "resourcequota",
				Namespace:    rq.Namespace,
				Name:         rq.Name,
				OldObject:    rq,
				Timestamp:    time.Now(),
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 ResourceQuota Informer 失败: %v", w.clusterUUID, err)
	}
}

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
					return
				}
			default:
				return
			}
			w.enqueueEvent(&ResourceEvent{
				Type:         EventDelete,
				ClusterUUID:  w.clusterUUID,
				ResourceType: "limitrange",
				Namespace:    lr.Namespace,
				Name:         lr.Name,
				OldObject:    lr,
				Timestamp:    time.Now(),
			})
		},
	})
	if err != nil {
		logx.Errorf("[ClusterWatcher] 集群 %s: 注册 LimitRange Informer 失败: %v", w.clusterUUID, err)
	}
}
