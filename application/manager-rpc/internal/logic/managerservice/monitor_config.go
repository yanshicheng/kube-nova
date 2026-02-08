package managerservicelogic

import "fmt"

// GenerateAlertmanagerSecretYAML 生成完整的 Alertmanager Secret YAML 配置
func GenerateAlertmanagerSecretYAML(namespace, name, webhookUrl, credentials string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  labels:
    app.kubernetes.io/component: alert-router
    app.kubernetes.io/instance: main
    app.kubernetes.io/name: alertmanager
    app.kubernetes.io/part-of: kube-prometheus
    app.kubernetes.io/version: 0.28.0
  name: %s
  namespace: %s
type: Opaque
stringData:
  alertmanager.yaml: |-
    global:
      resolve_timeout: 10m

    inhibit_rules:
      - equal:
          - namespace
          - alertname
        source_matchers:
          - severity = critical
        target_matchers:
          - severity =~ warning|info

      - equal:
          - namespace
          - alertname
        source_matchers:
          - severity = warning
        target_matchers:
          - severity = info

      - equal:
          - namespace
        source_matchers:
          - alertname = InfoInhibitor
        target_matchers:
          - severity = info

    receivers:
      - name: 'null'

      - name: 'webhook'
        webhook_configs:
          - url: '%s'
            send_resolved: true
            max_alerts: 0
            http_config:
              authorization:
                credentials: '%s'

    route:
      receiver: 'webhook'
      group_by: ['namespace', 'alertname']
      group_wait: 10s
      group_interval: 5m
      repeat_interval: 12h

      routes:
        - matchers:
            - alertname = Watch
          receiver: 'webhook'
          group_wait: 0s
          group_interval: 1m
          repeat_interval: 1m

        - matchers:
            - alertname = InfoInhibitor
          receiver: 'null'

        - matchers:
            - severity = critical
          receiver: 'webhook'
          group_wait: 30s
          group_interval: 10m
          repeat_interval: 4h
          continue: false

        - matchers:
            - severity = warning
          receiver: 'webhook'
          group_wait: 120s
          group_interval: 1h
          repeat_interval: 12h
          continue: false

        - matchers:
            - severity = info
          receiver: 'webhook'
          group_wait: 10m
          group_interval: 1h
          repeat_interval: 24h
          continue: false`, name, namespace, webhookUrl, credentials)
}

// GenerateAlertmanagerConfigMapYAML 生成完整的 Alertmanager ConfigMap YAML 配置
func GenerateAlertmanagerConfigMapYAML(namespace, name, webhookUrl, credentials string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    app.kubernetes.io/component: alert-router
    app.kubernetes.io/instance: main
    app.kubernetes.io/name: alertmanager
    app.kubernetes.io/part-of: kube-prometheus
    app.kubernetes.io/version: 0.28.0
  name: %s
  namespace: %s
data:
  alertmanager.yaml: |
    global:
      resolve_timeout: 10m

    inhibit_rules:
      - equal:
          - namespace
          - alertname
        source_matchers:
          - severity = critical
        target_matchers:
          - severity =~ warning|info

      - equal:
          - namespace
          - alertname
        source_matchers:
          - severity = warning
        target_matchers:
          - severity = info

      - equal:
          - namespace
        source_matchers:
          - alertname = InfoInhibitor
        target_matchers:
          - severity = info

    receivers:
      - name: 'null'

      - name: 'webhook'
        webhook_configs:
          - url: '%s'
            send_resolved: true
            max_alerts: 0
            http_config:
              authorization:
                credentials: '%s'

    route:
      receiver: 'webhook'
      group_by: ['namespace', 'alertname']
      group_wait: 10s
      group_interval: 5m
      repeat_interval: 12h

      routes:
        - matchers:
            - alertname = Watchdog
          receiver: 'webhook'
          group_wait: 0s
          group_interval: 1m
          repeat_interval: 1m

        - matchers:
            - alertname = InfoInhibitor
          receiver: 'null'

        - matchers:
            - severity = critical
          receiver: 'webhook'
          group_wait: 30s
          group_interval: 10m
          repeat_interval: 4h
          continue: false

        - matchers:
            - severity = warning
          receiver: 'webhook'
          group_wait: 120s
          group_interval: 1h
          repeat_interval: 12h
          continue: false

        - matchers:
            - severity = info
          receiver: 'webhook'
          group_wait: 10m
          group_interval: 1h
          repeat_interval: 24h
          continue: false`, name, namespace, webhookUrl, credentials)
}

// GenerateAlertmanagerConfigYAML 只生成 alertmanager.yaml 配置内容（不含 Secret/ConfigMap 外层）
func GenerateAlertmanagerConfigYAML(webhookUrl, credentials string) string {
	return fmt.Sprintf(`global:
  resolve_timeout: 10m

inhibit_rules:
  - equal:
      - namespace
      - alertname
    source_matchers:
      - severity = critical
    target_matchers:
      - severity =~ warning|info

  - equal:
      - namespace
      - alertname
    source_matchers:
      - severity = warning
    target_matchers:
      - severity = info

  - equal:
      - namespace
    source_matchers:
      - alertname = InfoInhibitor
    target_matchers:
      - severity = info

receivers:
  - name: 'null'

  - name: 'webhook'
    webhook_configs:
      - url: '%s'
        send_resolved: true
        max_alerts: 0
        http_config:
          authorization:
            credentials: '%s'

route:
  receiver: 'webhook'
  group_by: ['namespace', 'alertname']
  group_wait: 10s
  group_interval: 5m
  repeat_interval: 12h

  routes:
    - matchers:
        - alertname = Watch
      receiver: 'webhook'
      group_wait: 0s
      group_interval: 1m
      repeat_interval: 1m

    - matchers:
        - alertname = InfoInhibitor
      receiver: 'null'

    - matchers:
        - severity = critical
      receiver: 'webhook'
      group_wait: 30s
      group_interval: 10m
      repeat_interval: 4h
      continue: false

    - matchers:
        - severity = warning
      receiver: 'webhook'
      group_wait: 120s
      group_interval: 1h
      repeat_interval: 12h
      continue: false

    - matchers:
        - severity = info
      receiver: 'webhook'
      group_wait: 10m
      group_interval: 1h
      repeat_interval: 24h
      continue: false`, webhookUrl, credentials)
}

// PrometheusExternalLabels Prometheus 外部标签参数
type PrometheusExternalLabels struct {
	Environment string // 环境标识，如 "production"
	Cluster     string // 集群名称
	ClusterUuid string // 集群 UUID
	Region      string // 区域
	Datacenter  string // 数据中心，默认 "primary"
}

// 返回完整的 Prometheus CRD YAML 字符串
func GeneratePrometheusConfigYAML(namespace, name string, labels PrometheusExternalLabels) string {
	return fmt.Sprintf(`apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  labels:
    app.kubernetes.io/component: prometheus
    app.kubernetes.io/instance: %s
    app.kubernetes.io/name: prometheus
    app.kubernetes.io/part-of: kube-prometheus
    app.kubernetes.io/version: 3.4.0
  name: %s
  namespace: %s
spec:
  alerting:
    alertmanagers:
    - apiVersion: v2
      name: alertmanager-main
      namespace: %s
      port: web
      timeout: 10s
      pathPrefix: /

  enableFeatures:
    - "memory-snapshot-on-shutdown"
    - "delayed-compaction"
    - "concurrent-rule-eval"

  web:
    pageTitle: "Production Prometheus"
    maxConnections: 512

  externalLabels:
    environment: "%s"
    cluster: "%s"
    clusterUuid: "%s"
    region: "%s"
    datacenter: "%s"

  image: quay.io/prometheus/prometheus:v3.4.0

  nodeSelector:
    kubernetes.io/os: linux

  tolerations:
  - key: "monitoring"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
  - key: "node.kubernetes.io/disk-pressure"
    operator: "Exists"
    effect: "NoSchedule"

  affinity:
    podAntiAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        podAffinityTerm:
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: prometheus
              app.kubernetes.io/instance: %s
          topologyKey: kubernetes.io/hostname

  podMetadata:
    labels:
      app.kubernetes.io/component: prometheus
      app.kubernetes.io/instance: %s
      app.kubernetes.io/name: prometheus
      app.kubernetes.io/part-of: kube-prometheus
      app.kubernetes.io/version: 3.4.0
      tier: monitoring
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/port: "9090"
      config.linkerd.io/skip-outbound-ports: "9090"

  podMonitorNamespaceSelector: {}
  podMonitorSelector: {}

  probeNamespaceSelector: {}
  probeSelector: {}

  replicas: 2

  resources:
    requests:
      memory: 1Gi
      cpu: 100m
    limits:
      memory: 8Gi
      cpu: 4

  ruleNamespaceSelector: {}
  ruleSelector: {}

  scrapeConfigNamespaceSelector: {}
  scrapeConfigSelector: {}

  securityContext:
    fsGroup: 2000
    runAsNonRoot: true
    runAsUser: 1000
    fsGroupChangePolicy: "OnRootMismatch"
    seccompProfile:
      type: RuntimeDefault

  serviceAccountName: prometheus-k8s

  serviceMonitorNamespaceSelector: {}
  serviceMonitorSelector: {}

  version: 3.4.0

  retention: 7d
  retentionSize: 100GB

  storage:
    volumeClaimTemplate:
      spec:
        storageClassName: nfs-default
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 100Gi

  walCompression: true

  query:
    maxConcurrency: 20
    timeout: 2m
    lookbackDelta: 5m
    maxSamples: 50000000

  scrapeInterval: 30s
  scrapeTimeout: 5s
  evaluationInterval: 30s

  logLevel: info
  logFormat: logfmt

  enableAdminAPI: false

  additionalScrapeConfigs:
    name: additional-scrape-configs
    key: prometheus-additional.yaml

  terminationGracePeriodSeconds: 30

  enableOTLPReceiver: true

  otlp:
    promoteResourceAttributes:
      - "service.name"
      - "service.version"
      - "deployment.environment"
      - "k8s.namespace.name"
      - "k8s.pod.name"
      - "host.name"
    translationStrategy: "UnderscoreEscapingWithSuffixes"
    keepIdentifyingResourceAttributes: true
    convertHistogramsToNHCB: true

  containers:
  - name: prometheus
    env:
    - name: TZ
      value: "Asia/Shanghai"

  volumeMounts:
  - name: timezone
    mountPath: /etc/localtime
    readOnly: true

  volumes:
  - name: timezone
    hostPath:
      path: /usr/share/zoneinfo/Asia/Shanghai
      type: File`,
		name, name, namespace, namespace,
		labels.Environment, labels.Cluster, labels.ClusterUuid, labels.Region, labels.Datacenter,
		name, name)
}
