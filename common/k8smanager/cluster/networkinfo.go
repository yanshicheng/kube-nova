package cluster

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ClusterNetworkInfo struct {
	ClusterCIDR      string `json:"clusterCidr"`
	ServiceCIDR      string `json:"serviceCidr"`
	NodeCIDRMaskSize int    `json:"nodeCidrMaskSize"`

	DNSDomain    string `json:"dnsDomain"`
	DNSServiceIP string `json:"dnsServiceIp"`
	DNSProvider  string `json:"dnsProvider"`

	CNIPlugin  string `json:"cniPlugin"`
	CNIVersion string `json:"cniVersion"`
	ProxyMode  string `json:"proxyMode"`

	IngressController string `json:"ingressController"`
	IngressClass      string `json:"ingressClass"`

	IPv6Enabled      bool   `json:"ipv6Enabled"`
	DualStackEnabled bool   `json:"dualStackEnabled"`
	MTUSize          int    `json:"mtuSize"`
	EnableNodePort   bool   `json:"enableNodePort"`
	NodePortRange    string `json:"nodePortRange"`
}

type networkDetector interface {
	detect(ctx context.Context, info *ClusterNetworkInfo) error
	name() string
}

func (c *clusterClient) GetNetworkInfo() (*ClusterNetworkInfo, error) {
	ctx := context.Background()
	if atomic.LoadInt32(&c.closed) == 1 {
		return nil, fmt.Errorf("cluster %s is closed", c.config.ID)
	}

	detectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	info := c.initDefaultNetworkInfo()

	detectors := []networkDetector{
		&cidrDetector{client: c},
		&dnsDetector{client: c},
		&cniDetector{client: c},
		&proxyDetector{client: c},
		&ingressDetector{client: c},
		&ipFamilyDetector{client: c},
		&nodePortDetector{client: c},
	}

	if err := c.runDetectors(detectCtx, detectors, info); err != nil {
		c.l.Errorf("部分网络配置检测失败: %v", err)
	}

	c.l.Infof("成功获取集群 %s 网络配置信息", c.config.ID)
	return info, nil
}

func (c *clusterClient) initDefaultNetworkInfo() *ClusterNetworkInfo {
	return &ClusterNetworkInfo{
		ClusterCIDR:       "10.244.0.0/16",
		ServiceCIDR:       "10.96.0.0/12",
		NodeCIDRMaskSize:  24,
		DNSDomain:         "cluster.local",
		DNSServiceIP:      "10.96.0.10",
		DNSProvider:       "coredns",
		CNIPlugin:         "unknown",
		CNIVersion:        "",
		ProxyMode:         "iptables",
		IngressController: "unknown",
		IngressClass:      "",
		IPv6Enabled:       false,
		DualStackEnabled:  false,
		MTUSize:           1500,
		EnableNodePort:    true,
		NodePortRange:     "30000-32767",
	}
}

func (c *clusterClient) runDetectors(ctx context.Context, detectors []networkDetector, info *ClusterNetworkInfo) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(detectors))

	for _, detector := range detectors {
		wg.Add(1)
		go func(d networkDetector) {
			defer wg.Done()
			if err := d.detect(ctx, info); err != nil {
				c.l.Errorf("检测器 %s 执行失败: %v", d.name(), err)
				errChan <- fmt.Errorf("%s: %w", d.name(), err)
			}
		}(detector)
	}

	wg.Wait()
	close(errChan)

	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("检测器错误: %d/%d 失败", len(errs), len(detectors))
	}
	return nil
}

type cidrDetector struct {
	client *clusterClient
}

func (d *cidrDetector) name() string {
	return "CIDR"
}

func (d *cidrDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	nodes, err := d.client.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 5})
	if err == nil && len(nodes.Items) > 0 {
		for _, node := range nodes.Items {
			if len(node.Spec.PodCIDRs) > 0 {
				info.ClusterCIDR = strings.Join(node.Spec.PodCIDRs, ",")
				for _, cidr := range node.Spec.PodCIDRs {
					if !strings.Contains(cidr, ":") {
						parts := strings.Split(cidr, "/")
						if len(parts) == 2 {
							fmt.Sscanf(parts[1], "%d", &info.NodeCIDRMaskSize)
						}
						break
					}
				}
				break
			}
			if node.Spec.PodCIDR != "" {
				info.ClusterCIDR = node.Spec.PodCIDR
				parts := strings.Split(node.Spec.PodCIDR, "/")
				if len(parts) == 2 {
					fmt.Sscanf(parts[1], "%d", &info.NodeCIDRMaskSize)
				}
				break
			}
		}
	}

	cm, err := d.client.kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kubeadm-config", metav1.GetOptions{})
	if err == nil {
		if clusterStatus, ok := cm.Data["ClusterConfiguration"]; ok {
			if idx := strings.Index(clusterStatus, "serviceSubnet:"); idx != -1 {
				sub := clusterStatus[idx:]
				if endIdx := strings.Index(sub, "\n"); endIdx != -1 {
					val := strings.TrimSpace(strings.TrimPrefix(sub[:endIdx], "serviceSubnet:"))
					if val != "" {
						info.ServiceCIDR = val
					}
				}
			}
		}
	}

	return nil
}

type dnsDetector struct {
	client *clusterClient
}

func (d *dnsDetector) name() string {
	return "DNS"
}

func (d *dnsDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	dnsServiceNames := []string{"kube-dns", "coredns"}
	for _, name := range dnsServiceNames {
		svc, err := d.client.kubeClient.CoreV1().Services("kube-system").Get(ctx, name, metav1.GetOptions{})
		if err == nil && svc.Spec.ClusterIP != "" && svc.Spec.ClusterIP != "None" {
			info.DNSServiceIP = svc.Spec.ClusterIP
			info.DNSProvider = name
			break
		}
	}

	if info.DNSProvider == "coredns" {
		cm, err := d.client.kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "coredns", metav1.GetOptions{})
		if err == nil {
			if corefile, ok := cm.Data["Corefile"]; ok {
				lines := strings.Split(corefile, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "kubernetes") {
						parts := strings.Fields(line)
						if len(parts) >= 2 {
							info.DNSDomain = parts[1]
							break
						}
					}
				}
			}
		}
	}
	return nil
}

type cniDetector struct {
	client *clusterClient
}

func (d *cniDetector) name() string {
	return "CNI"
}

func (d *cniDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	cniDetectors := map[string][]string{
		"calico":  {"calico-node", "calico", "calico-kube-controllers"},
		"flannel": {"kube-flannel", "kube-flannel-ds", "flannel"},
		"cilium":  {"cilium", "cilium-agent"},
	}

	daemonsets, err := d.client.kubeClient.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("获取DaemonSet列表失败: %w", err)
	}

	for _, ds := range daemonsets.Items {
		dsName := strings.ToLower(ds.Name)
		for plugin, patterns := range cniDetectors {
			for _, pattern := range patterns {
				if strings.Contains(dsName, pattern) {
					info.CNIPlugin = plugin
					if len(ds.Spec.Template.Spec.Containers) > 0 {
						image := ds.Spec.Template.Spec.Containers[0].Image
						info.CNIVersion = extractVersionFromImage(image)
					}
					if cmName, ok := getCNIConfigMapName(plugin); ok {
						d.detectMTUFromConfigMap(ctx, cmName, ds.Namespace, info)
					}
					return nil
				}
			}
		}
	}
	info.CNIPlugin = "unknown"
	return nil
}

func (d *cniDetector) detectMTUFromConfigMap(ctx context.Context, cmName string, namespace string, info *ClusterNetworkInfo) {
	if namespace == "" {
		namespace = "kube-system"
	}
	cm, err := d.client.kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return
	}
	for _, data := range cm.Data {
		if strings.Contains(data, "mtu") {
			fmt.Sscanf(data, "mtu: %d", &info.MTUSize)
		}
	}
}

type proxyDetector struct {
	client *clusterClient
}

func (d *proxyDetector) name() string {
	return "Proxy"
}

func (d *proxyDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	cm, err := d.client.kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "kube-proxy", metav1.GetOptions{})
	if err != nil {
		return nil
	}

	configData := ""
	if config, ok := cm.Data["config.conf"]; ok {
		configData = config
	} else if config, ok := cm.Data["config"]; ok {
		configData = config
	}

	if configData == "" {
		return nil
	}

	configLower := strings.ToLower(configData)
	switch {
	case strings.Contains(configLower, "mode: ipvs") || strings.Contains(configLower, `mode: "ipvs"`):
		info.ProxyMode = "ipvs"
	case strings.Contains(configLower, "mode: iptables") || strings.Contains(configLower, `mode: "iptables"`):
		info.ProxyMode = "iptables"
	case strings.Contains(configLower, "mode: userspace"):
		info.ProxyMode = "userspace"
	default:
		info.ProxyMode = "iptables"
	}
	return nil
}

type ingressDetector struct {
	client *clusterClient
}

func (d *ingressDetector) name() string {
	return "Ingress"
}

func (d *ingressDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	ingressDetectors := map[string][]string{
		"nginx":   {"ingress-nginx", "nginx-ingress"},
		"traefik": {"traefik"},
		"haproxy": {"haproxy-ingress"},
		"istio":   {"istio-ingressgateway"},
		"kong":    {"kong"},
		"apisix":  {"apisix-ingress"},
		"contour": {"contour"},
	}

	foundControllers := make(map[string]bool)

	deployments, err := d.client.kubeClient.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, deploy := range deployments.Items {
			if controller := matchIngressController(deploy.Name, ingressDetectors); controller != "" {
				foundControllers[controller] = true
			}
		}
	}

	daemonsets, err := d.client.kubeClient.AppsV1().DaemonSets("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ds := range daemonsets.Items {
			if controller := matchIngressController(ds.Name, ingressDetectors); controller != "" {
				foundControllers[controller] = true
			}
		}
	}

	if len(foundControllers) > 0 {
		controllers := make([]string, 0, len(foundControllers))
		for controller := range foundControllers {
			controllers = append(controllers, controller)
		}
		info.IngressController = strings.Join(controllers, ",")
	} else {
		info.IngressController = "unknown"
	}

	ingressClasses, err := d.client.kubeClient.NetworkingV1().IngressClasses().List(ctx, metav1.ListOptions{})
	if err == nil && len(ingressClasses.Items) > 0 {
		var defaultClass string
		var allClasses []string

		for _, ic := range ingressClasses.Items {
			allClasses = append(allClasses, ic.Name)
			if ic.Annotations["ingressclass.kubernetes.io/is-default-class"] == "true" {
				defaultClass = ic.Name
			}
		}

		if defaultClass != "" {
			info.IngressClass = defaultClass
		} else if len(allClasses) > 0 {
			info.IngressClass = strings.Join(allClasses, ",")
		}
	}

	return nil
}

type ipFamilyDetector struct {
	client *clusterClient
}

func (d *ipFamilyDetector) name() string {
	return "IPFamily"
}

func (d *ipFamilyDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	// 从 Node 的 PodCIDRs 判断双栈和 IPv6
	nodes, err := d.client.kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 5})
	if err == nil && len(nodes.Items) > 0 {
		for _, node := range nodes.Items {
			if len(node.Spec.PodCIDRs) > 1 {
				info.DualStackEnabled = true
			}
			for _, cidr := range node.Spec.PodCIDRs {
				if strings.Contains(cidr, ":") {
					info.IPv6Enabled = true
					break
				}
			}
			if info.DualStackEnabled || info.IPv6Enabled {
				break
			}
		}
	}

	// 从 kube-apiserver 参数检测双栈
	pods, err := d.client.kubeClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "component=kube-apiserver",
	})
	if err == nil && len(pods.Items) > 0 {
		for _, container := range pods.Items[0].Spec.Containers {
			for _, arg := range container.Command {
				if strings.Contains(arg, "--service-cluster-ip-range") && strings.Contains(arg, ",") {
					info.DualStackEnabled = true
				}
			}
		}
	}

	// 从 Service 检测
	namespaces := []string{"default", "kube-system"}
	for _, ns := range namespaces {
		services, err := d.client.kubeClient.CoreV1().Services(ns).List(ctx, metav1.ListOptions{Limit: 20})
		if err != nil {
			continue
		}
		for _, svc := range services.Items {
			if !info.IPv6Enabled && isIPv6Address(svc.Spec.ClusterIP) {
				info.IPv6Enabled = true
			}
			if !info.DualStackEnabled {
				if svc.Spec.IPFamilyPolicy != nil &&
					(*svc.Spec.IPFamilyPolicy == corev1.IPFamilyPolicyRequireDualStack ||
						*svc.Spec.IPFamilyPolicy == corev1.IPFamilyPolicyPreferDualStack) {
					info.DualStackEnabled = true
				}
				if len(svc.Spec.IPFamilies) > 1 {
					info.DualStackEnabled = true
				}
				if len(svc.Spec.ClusterIPs) > 1 {
					info.DualStackEnabled = true
				}
			}
			if info.IPv6Enabled && info.DualStackEnabled {
				return nil
			}
		}
	}

	return nil
}

type nodePortDetector struct {
	client *clusterClient
}

func (d *nodePortDetector) name() string {
	return "NodePort"
}

func (d *nodePortDetector) detect(ctx context.Context, info *ClusterNetworkInfo) error {
	info.EnableNodePort = true
	info.NodePortRange = "30000-32767"

	// 从 kube-apiserver 参数获取 NodePort 范围
	pods, err := d.client.kubeClient.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
		LabelSelector: "component=kube-apiserver",
	})
	if err == nil && len(pods.Items) > 0 {
		for _, container := range pods.Items[0].Spec.Containers {
			for _, arg := range container.Command {
				if strings.HasPrefix(arg, "--service-node-port-range=") {
					info.NodePortRange = strings.TrimPrefix(arg, "--service-node-port-range=")
					return nil
				}
			}
		}
	}

	// 从现有 Service 推断端口范围
	services, err := d.client.kubeClient.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	minPort := 32767
	maxPort := 30000

	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeNodePort || svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			for _, port := range svc.Spec.Ports {
				if port.NodePort > 0 {
					nodePort := int(port.NodePort)
					if nodePort < minPort {
						minPort = nodePort
					}
					if nodePort > maxPort {
						maxPort = nodePort
					}
				}
			}
		}
	}

	if minPort <= maxPort && minPort != 32767 {
		info.NodePortRange = fmt.Sprintf("%d-%d", minPort, maxPort)
	}

	return nil
}

func extractVersionFromImage(image string) string {
	if strings.Contains(image, "@") {
		return "digest"
	}
	if strings.Contains(image, ":") {
		parts := strings.Split(image, ":")
		tag := parts[len(parts)-1]
		if tag != "latest" && tag != "main" && tag != "master" {
			return tag
		}
	}
	return ""
}

func getCNIConfigMapName(plugin string) (string, bool) {
	configMaps := map[string]string{
		"calico":  "calico-config",
		"flannel": "kube-flannel-cfg",
		"cilium":  "cilium-config",
	}
	name, ok := configMaps[plugin]
	return name, ok
}

func matchIngressController(name string, detectors map[string][]string) string {
	nameLower := strings.ToLower(name)
	for controller, patterns := range detectors {
		for _, pattern := range patterns {
			if strings.Contains(nameLower, pattern) {
				return controller
			}
		}
	}
	return ""
}

func isIPv6Address(ip string) bool {
	return strings.Contains(ip, ":") && ip != "::" && ip != "None"
}
