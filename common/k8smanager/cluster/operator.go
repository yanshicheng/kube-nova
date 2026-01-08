package cluster

import (
	"github.com/yanshicheng/kube-nova/common/k8smanager/operator"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
)

type operatorInterface interface {
	// 获取命名空间操作器
	Namespaces() types.NamespaceOperator
	LimitRange() types.LimitRangeOperator
	ResourceQuota() types.ResourceQuotaOperator
	// 获取 Pod 操作器
	Pods() types.PodOperator
	Node() types.NodeOperator

	// 工作负载控制器
	Deployment() types.DeploymentOperator
	StatefulSet() types.StatefulSetOperator
	DaemonSet() types.DaemonSetOperator
	CronJob() types.CronJobOperator
	Job() types.JobOperator
	ReplicaSet() types.ReplicaSetOperator

	// 配置管理
	ConfigMaps() types.ConfigMapOperator
	Secrets() types.SecretOperator
	Services() types.ServiceOperator
	Events() types.EventOperator
	Ingresses() types.IngressOperator
	IngressClasses() types.IngressClassOperator
	PVC() types.PVCOperator
	ServiceAccounts() types.ServiceAccountOperator
	Roles() types.RoleOperator
	RoleBindings() types.RoleBindingOperator

	// autoscaling相关
	HPA() types.HPAOperator
	VPA() types.VPAOperator

	// canary 相关
	Flagger() types.FlaggerOperator

	// 集群相关
	StorageClasses() types.StorageClassOperator
	ClusterRoles() types.ClusterRoleOperator
	ClusterRoleBindings() types.ClusterRoleBindingOperator
	PersistentVolumes() types.PersistentVolumeOperator
	// ========== 监控相关 ==========
	// Probe 获取 Probe 操作器
	Probe() types.ProbeOperator
	// PrometheusRule 获取 PrometheusRule 操作器
	PrometheusRule() types.PrometheusRuleOperator
}

type resourceOperator struct {
	// 资源操作器
	namespaces      types.NamespaceOperator
	limitRange      types.LimitRangeOperator
	resourceQuota   types.ResourceQuotaOperator
	pods            types.PodOperator
	node            types.NodeOperator
	deployment      types.DeploymentOperator
	statefulSet     types.StatefulSetOperator
	daemonSet       types.DaemonSetOperator
	cronJob         types.CronJobOperator
	job             types.JobOperator
	replicaSet      types.ReplicaSetOperator
	configMaps      types.ConfigMapOperator
	secrets         types.SecretOperator
	services        types.ServiceOperator
	events          types.EventOperator
	ingress         types.IngressOperator
	ingressClasses  types.IngressClassOperator
	pvc             types.PVCOperator
	hpa             types.HPAOperator
	vpa             types.VPAOperator
	flagger         types.FlaggerOperator
	storageClasses  types.StorageClassOperator
	serviceAccounts types.ServiceAccountOperator
	roles           types.RoleOperator
	roleBindings    types.RoleBindingOperator

	clusterRole        types.ClusterRoleOperator
	clusterRoleBinding types.ClusterRoleBindingOperator
	persistentVolume   types.PersistentVolumeOperator
	// ========== 监控相关 ==========
	probe          types.ProbeOperator
	prometheusRule types.PrometheusRuleOperator
}

// initOperators 初始化所有操作器
func (c *clusterClient) initOperators() {
	//根据是否启用 Informer 创建不同的操作器
	if c.config.Options.EnableInformer && c.informerManager != nil {
		// 使用带 Informer 的操作器，传入上下文
		c.namespaces = operator.NewNamespaceOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)

		// 创建带 Informer 的 Pod 操作器
		c.pods = operator.NewPodOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.restConfig,
			c.informerManager.GetInformerFactory(),
		)
		c.node = operator.NewNodeOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.restConfig,
			c.informerManager.GetInformerFactory(),
		)
		c.deployment = operator.NewDeploymentOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.statefulSet = operator.NewStatefulSetOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.daemonSet = operator.NewDaemonSetOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.cronJob = operator.NewCronJobOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.job = operator.NewJobOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.replicaSet = operator.NewReplicaSetOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.configMaps = operator.NewConfigMapOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.secrets = operator.NewSecretOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.services = operator.NewServiceOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.events = operator.NewEventOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.ingress = operator.NewIngressOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.ingressClasses = operator.NewIngressClassOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.pvc = operator.NewPVCOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.limitRange = operator.NewLimitRangeOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.resourceQuota = operator.NewResourceQuotaOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.hpa = operator.NewHPAOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.vpa = operator.NewVPAOperatorWithInformer(
			c.ctx,
			c.vpaClientset,       // 使用 VPA 专用 clientset
			c.kubeClient,         // 传入 k8s client
			c.vpaInformerFactory, // 使用 VPA InformerFactory
		)

		c.flagger = operator.NewFlaggerOperatorWithInformer(
			c.ctx,
			c.flaggerClientset,       // 使用 Flagger 专用 clientset
			c.flaggerInformerFactory, // 使用 Flagger InformerFactory
		)
		c.storageClasses = operator.NewStorageClassOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.clusterRole = operator.NewClusterRoleOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.clusterRoleBinding = operator.NewClusterRoleBindingOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.persistentVolume = operator.NewPersistentVolumeOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.serviceAccounts = operator.NewServiceAccountOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.roles = operator.NewRoleOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.roleBindings = operator.NewRoleBindingOperatorWithInformer(
			c.ctx,
			c.kubeClient,
			c.informerManager.GetInformerFactory(),
		)
		c.probe = operator.NewProbeOperatorWithInformer(
			c.ctx,
			c.monitoringClientset,
			c.monitoringInformerFactory,
		)
		c.prometheusRule = operator.NewPrometheusRuleOperatorWithInformer(
			c.ctx,
			c.monitoringClientset,
			c.monitoringInformerFactory,
		)

	} else {
		// 使用普通操作器，传入上下文
		c.namespaces = operator.NewNamespaceOperator(c.ctx, c.kubeClient)

		// 创建普通 Pod 操作器
		c.pods = operator.NewPodOperator(c.ctx, c.kubeClient, c.restConfig)
		c.node = operator.NewNodeOperator(c.ctx, c.kubeClient, c.restConfig)
		c.deployment = operator.NewDeploymentOperator(c.ctx, c.kubeClient)
		c.statefulSet = operator.NewStatefulSetOperator(c.ctx, c.kubeClient)
		c.daemonSet = operator.NewDaemonSetOperator(c.ctx, c.kubeClient)
		c.cronJob = operator.NewCronJobOperator(c.ctx, c.kubeClient)
		c.job = operator.NewJobOperator(c.ctx, c.kubeClient)
		c.replicaSet = operator.NewReplicaSetOperator(c.ctx, c.kubeClient)
		c.configMaps = operator.NewConfigMapOperator(c.ctx, c.kubeClient)
		c.secrets = operator.NewSecretOperator(c.ctx, c.kubeClient)
		c.services = operator.NewServiceOperator(c.ctx, c.kubeClient)
		c.events = operator.NewEventOperator(c.ctx, c.kubeClient)
		c.ingress = operator.NewIngressOperator(c.ctx, c.kubeClient)
		c.ingressClasses = operator.NewIngressClassOperator(c.ctx, c.kubeClient)
		c.pvc = operator.NewPVCOperator(c.ctx, c.kubeClient)
		c.limitRange = operator.NewLimitRangeOperator(c.ctx, c.kubeClient)
		c.resourceQuota = operator.NewResourceQuotaOperator(c.ctx, c.kubeClient)
		c.hpa = operator.NewHPAOperator(c.ctx, c.kubeClient)
		c.vpa = operator.NewVPAOperator(
			c.ctx,
			c.vpaClientset, // 使用 VPA 专用 clientset
			c.kubeClient,   // 传入 k8s client
		)

		c.flagger = operator.NewFlaggerOperator(
			c.ctx,
			c.flaggerClientset, // 使用 Flagger 专用 clientset
		)
		c.storageClasses = operator.NewStorageClassOperator(c.ctx, c.kubeClient)
		c.clusterRole = operator.NewClusterRoleOperator(c.ctx, c.kubeClient)
		c.clusterRoleBinding = operator.NewClusterRoleBindingOperator(c.ctx, c.kubeClient)
		c.persistentVolume = operator.NewPersistentVolumeOperator(c.ctx, c.kubeClient)
		c.serviceAccounts = operator.NewServiceAccountOperator(c.ctx, c.kubeClient)
		c.roles = operator.NewRoleOperator(c.ctx, c.kubeClient)
		c.roleBindings = operator.NewRoleBindingOperator(c.ctx, c.kubeClient)
		c.probe = operator.NewProbeOperator(c.ctx, c.monitoringClientset)
		c.prometheusRule = operator.NewPrometheusRuleOperator(c.ctx, c.monitoringClientset)
	}
}

// Namespaces 获取命名空间操作器
func (c *clusterClient) Namespaces() types.NamespaceOperator {
	return c.namespaces
}

// Pods 获取 Pod 操作器
func (c *clusterClient) Pods() types.PodOperator {
	return c.pods
}

// Node 获取 Node 操作器
func (c *clusterClient) Node() types.NodeOperator {
	return c.node
}

// Deployment 获取 Deployment 操作器
func (c *clusterClient) Deployment() types.DeploymentOperator {
	return c.deployment
}

// StatefulSet 获取 StatefulSet 操作器
func (c *clusterClient) StatefulSet() types.StatefulSetOperator {
	return c.statefulSet
}

// DaemonSet 获取 DaemonSet 操作器
func (c *clusterClient) DaemonSet() types.DaemonSetOperator {
	return c.daemonSet
}

// CronJob 获取 CronJob 操作器
func (c *clusterClient) CronJob() types.CronJobOperator {
	return c.cronJob
}

// Job 获取 Job 操作器
func (c *clusterClient) Job() types.JobOperator {
	return c.job
}

// ReplicaSet 获取 ReplicaSet 操作器
func (c *clusterClient) ReplicaSet() types.ReplicaSetOperator {
	return c.replicaSet
}

// ConfigMaps 获取 ConfigMaps 操作器
func (c *clusterClient) ConfigMaps() types.ConfigMapOperator {
	return c.configMaps
}

// Secrets 获取 Secrets 操作器
func (c *clusterClient) Secrets() types.SecretOperator {
	return c.secrets
}

// Service 获取 Service 操作器
func (c *clusterClient) Services() types.ServiceOperator {
	return c.services
}

// Events 获取 Events 操作器
func (c *clusterClient) Events() types.EventOperator {
	return c.events
}

// Ingresses 获取 Ingress 操作器
func (c *clusterClient) Ingresses() types.IngressOperator {
	return c.ingress
}

// IngressClasses 获取 IngressClasses 操作器
func (c *clusterClient) IngressClasses() types.IngressClassOperator {
	return c.ingressClasses
}

// PVC 获取 PVC 操作器
func (c *clusterClient) PVC() types.PVCOperator {
	return c.pvc
}

// LimitRange 获取 LimitRange 操作器
func (c *clusterClient) LimitRange() types.LimitRangeOperator {
	return c.limitRange
}

// ResourceQuota 获取 ResourceQuota 操作器
func (c *clusterClient) ResourceQuota() types.ResourceQuotaOperator {
	return c.resourceQuota
}

func (c *clusterClient) HPA() types.HPAOperator {
	return c.hpa
}

func (c *clusterClient) VPA() types.VPAOperator {
	return c.vpa
}

func (c *clusterClient) Flagger() types.FlaggerOperator {
	return c.flagger
}
func (c *clusterClient) StorageClasses() types.StorageClassOperator {
	return c.storageClasses
}
func (c *clusterClient) ClusterRoles() types.ClusterRoleOperator {
	return c.clusterRole
}
func (c *clusterClient) ClusterRoleBindings() types.ClusterRoleBindingOperator {
	return c.clusterRoleBinding
}
func (c *clusterClient) PersistentVolumes() types.PersistentVolumeOperator {
	return c.persistentVolume
}

func (c *clusterClient) ServiceAccounts() types.ServiceAccountOperator {
	return c.serviceAccounts
}

func (c *clusterClient) Roles() types.RoleOperator {
	return c.roles
}

func (c *clusterClient) RoleBindings() types.RoleBindingOperator {
	return c.roleBindings
}

// Probe 获取 Probe 操作器
func (c *clusterClient) Probe() types.ProbeOperator {
	return c.probe
}

// PrometheusRule 获取 PrometheusRule 操作器
func (c *clusterClient) PrometheusRule() types.PrometheusRuleOperator {
	return c.prometheusRule
}
