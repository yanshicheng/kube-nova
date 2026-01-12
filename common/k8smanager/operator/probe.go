package operator

import (
	"context"
	"fmt"
	"strings"
	"time"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	monitoringinformers "github.com/prometheus-operator/prometheus-operator/pkg/client/informers/externalversions"
	monitoringlister "github.com/prometheus-operator/prometheus-operator/pkg/client/listers/monitoring/v1"
	monitoringclient "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned"
	"github.com/yanshicheng/kube-nova/common/k8smanager/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/yaml"
)

type probeOperator struct {
	BaseOperator
	client          monitoringclient.Interface
	informerFactory monitoringinformers.SharedInformerFactory
	probeLister     monitoringlister.ProbeLister
}

// NewProbeOperator 创建 Probe 操作器（不使用 informer）
func NewProbeOperator(ctx context.Context, client monitoringclient.Interface) types.ProbeOperator {
	return &probeOperator{
		BaseOperator: NewBaseOperator(ctx, false),
		client:       client,
	}
}

// NewProbeOperatorWithInformer 创建 Probe 操作器（使用 informer）
func NewProbeOperatorWithInformer(
	ctx context.Context,
	client monitoringclient.Interface,
	informerFactory monitoringinformers.SharedInformerFactory,
) types.ProbeOperator {
	var probeLister monitoringlister.ProbeLister

	if informerFactory != nil {
		probeLister = informerFactory.Monitoring().V1().Probes().Lister()
	}

	return &probeOperator{
		BaseOperator:    NewBaseOperator(ctx, informerFactory != nil),
		client:          client,
		informerFactory: informerFactory,
		probeLister:     probeLister,
	}
}

// Create 创建 Probe
func (p *probeOperator) Create(probe *monitoringv1.Probe) error {
	if probe == nil || probe.Name == "" {
		return fmt.Errorf("Probe 不能为空")
	}
	injectCommonAnnotations(probe)
	// 判断是否已经存在
	_, err := p.client.MonitoringV1().Probes(probe.Namespace).Get(p.ctx, probe.Name, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("Probe %s/%s 已经存在", probe.Namespace, probe.Name)
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("检查 Probe 是否存在失败: %v", err)
	}

	_, err = p.client.MonitoringV1().Probes(probe.Namespace).Create(p.ctx, probe, metav1.CreateOptions{})
	return err
}

// Get 获取 Probe 的 YAML
func (p *probeOperator) Get(namespace, name string) (string, error) {
	if namespace == "" || name == "" {
		return "", fmt.Errorf("命名空间和名称不能为空")
	}

	var probe *monitoringv1.Probe
	var err error

	// 优先使用 informer
	if p.useInformer && p.probeLister != nil {
		probe, err = p.probeLister.Probes(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("Probe %s/%s 不存在", namespace, name)
			}
			// informer 出错时回退到 API 调用
			probe, err = p.client.MonitoringV1().Probes(namespace).Get(p.ctx, name, metav1.GetOptions{})
			if err != nil {
				return "", fmt.Errorf("获取 Probe 失败: %v", err)
			}
		} else {
			// 返回副本以避免修改缓存
			probe = probe.DeepCopy()
		}
	} else {
		// 使用 API 调用
		probe, err = p.client.MonitoringV1().Probes(namespace).Get(p.ctx, name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return "", fmt.Errorf("Probe %s/%s 不存在", namespace, name)
			}
			return "", fmt.Errorf("获取 Probe 失败: %v", err)
		}
	}

	// 注入 TypeMeta
	p.injectTypeMeta(probe)

	// 清理 ManagedFields
	probe.ManagedFields = nil

	// 转换为 YAML
	yamlBytes, err := yaml.Marshal(probe)
	if err != nil {
		return "", fmt.Errorf("转换为 YAML 失败: %v", err)
	}

	return string(yamlBytes), nil
}

// List 获取 Probe 列表
func (p *probeOperator) List(namespace string, search string) (*types.ListProbeResponse, error) {
	var probes []*monitoringv1.Probe
	var err error

	// 优先使用 informer
	if p.useInformer && p.probeLister != nil {
		probes, err = p.probeLister.Probes(namespace).List(labels.Everything())
		if err != nil {
			return nil, fmt.Errorf("获取 Probe 列表失败: %v", err)
		}
	} else {
		// 使用 API 调用
		probeList, err := p.client.MonitoringV1().Probes(namespace).List(p.ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("获取 Probe 列表失败: %v", err)
		}
		probes = make([]*monitoringv1.Probe, len(probeList.Items))
		for i := range probeList.Items {
			probes[i] = &probeList.Items[i]
		}
	}

	// 搜索过滤
	if search != "" {
		filtered := make([]*monitoringv1.Probe, 0)
		searchLower := strings.ToLower(search)
		for _, probe := range probes {
			if strings.Contains(strings.ToLower(probe.Name), searchLower) {
				filtered = append(filtered, probe)
			}
		}
		probes = filtered
	}

	// 转换为响应格式
	items := make([]types.ProbeInfo, len(probes))
	for i, probe := range probes {
		items[i] = types.ProbeInfo{
			Name:              probe.Name,
			Namespace:         probe.Namespace,
			Age:               p.formatAge(probe.CreationTimestamp.Time),
			Labels:            probe.Labels,
			Annotations:       probe.Annotations,
			CreationTimestamp: probe.CreationTimestamp.UnixMilli(),
		}
	}

	return &types.ListProbeResponse{
		Total: len(items),
		Items: items,
	}, nil
}

// Update 更新 Probe
func (p *probeOperator) Update(namespace, name string, probe *monitoringv1.Probe) error {
	if namespace == "" || name == "" || probe == nil {
		return fmt.Errorf("命名空间、名称和 Probe 不能为空")
	}

	// 验证名称和命名空间
	if probe.Name != name {
		return fmt.Errorf("Probe 名称(%s)与参数名称(%s)不匹配", probe.Name, name)
	}
	if probe.Namespace != "" && probe.Namespace != namespace {
		return fmt.Errorf("Probe 命名空间(%s)与参数命名空间(%s)不匹配", probe.Namespace, namespace)
	}
	probe.Namespace = namespace

	// 获取现有的 Probe 以保留 ResourceVersion
	existing, err := p.client.MonitoringV1().Probes(namespace).Get(p.ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("获取现有 Probe 失败: %v", err)
	}

	probe.ResourceVersion = existing.ResourceVersion

	// 更新
	_, err = p.client.MonitoringV1().Probes(namespace).Update(p.ctx, probe, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("更新 Probe 失败: %v", err)
	}

	return nil
}

// Delete 删除 Probe
func (p *probeOperator) Delete(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("命名空间和名称不能为空")
	}

	err := p.client.MonitoringV1().Probes(namespace).Delete(p.ctx, name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("删除 Probe 失败: %v", err)
	}

	return nil
}

// injectTypeMeta 注入 TypeMeta
func (p *probeOperator) injectTypeMeta(probe *monitoringv1.Probe) {
	probe.TypeMeta = metav1.TypeMeta{
		Kind:       "Probe",
		APIVersion: "monitoring.coreos.com/v1",
	}
}

// formatAge 格式化时间
func (p *probeOperator) formatAge(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
	if duration.Hours() >= 1 {
		return fmt.Sprintf("%dh", int(duration.Hours()))
	}
	if duration.Minutes() >= 1 {
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	}
	return fmt.Sprintf("%ds", int(duration.Seconds()))
}
