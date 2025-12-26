<p align="center">   <img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License">   <img src="https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go&logoColor=white" alt="Go">   <img src="https://img.shields.io/badge/Gozero-v1.9.4-1E88E5" alt="Gozero">   <img src="https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js&logoColor=white" alt="Vue">   <img src="https://img.shields.io/badge/Kubernetes-1.21+-326CE5?logo=kubernetes&logoColor=white" alt="Kubernetes">   <br/>   <img src="https://img.shields.io/badge/MySQL-8.0+-4479A1?logo=mysql&logoColor=white" alt="MySQL">   <img src="https://img.shields.io/badge/Redis-7.0+-DC382D?logo=redis&logoColor=white" alt="Redis">   <a target="_blank" href="https://github.com/yanshicheng/kube-nova">     <img src="https://img.shields.io/github/stars/yanshicheng/kube-nova?style=social" alt="GitHub Stars">   </a> </p> <p align="center">   <b>企业级 Kubernetes 多集群管理平台</b> </p> <p align="center">   <a href="https://www.ikubeops.com"><b>🌐 在线演示</b></a> |   <a href="https://www.cnblogs.com/yanshicheng/articles/19363328"><b>📖 部署文档</b></a> |   <a href="https://wiki-images.yanshicheng.com/common/kube-nova-wechat.png"><b>💬 加入我们</b></a> </p>

------

## 📦 代码仓库

| 仓库       | GitHub                                                                              | Gitee                                                                        |
|----------|-------------------------------------------------------------------------------------|------------------------------------------------------------------------------|
| 后端代码     | [yanshicheng/kube-nova](https://github.com/yanshicheng/kube-nova)                   | [ikubeops/kube-nova](https://gitee.com/ikubeops/kube-nova)                   |
| 前端代码     | [yanshicheng/kube-nova-web](https://github.com/yanshicheng/kube-nova-web)           | [ikubeops/kube-nova-web](https://gitee.com/ikubeops/kube-nova-web)           |
| operator | [yanshicheng/kube-nova-operator](https://github.com/yanshicheng/kube-nova-operator) | [ikubeops/kube-nova-operator](https://gitee.com/ikubeops/kube-nova-operator) |
------

## 📖 项目简介

Kube-Nova 是一个企业级 Kubernetes 多集群管理平台，以**项目为视角**实现多租户、多集群的资源隔离与统一管理。平台采用**项目 → 集群配额 → 工作空间**三级架构，提供完整的 RBAC 权限体系、资源超分、告警中心、灰度发布等企业级特性。

### 🎯 核心亮点

- 🏢 **多租户隔离** - 三级资源隔离架构，支持 CPU/内存/GPU 超分配置
- 🌐 **多集群管理** - 统一管理多个 K8s 集群，支持多种认证方式
- 📊 **五维监控** - Pod/Node/Namespace/Cluster/Ingress 全方位监控
- 🔔 **智能告警** - 8 种通知渠道，支持告警分组、分级路由
- 🚀 **灰度发布** - 基于 Flagger 的自动化金丝雀部署
- 📦 **镜像管理** - Harbor 多仓库统一管理
- 🔍 **完整审计** - 四级审计日志（集群/项目/工作空间/应用）
- 🛠️ **Pod 运维** - 日志查看、交互终端、文件管理

------

## 🏗️ 技术架构

### 后端技术栈

- **语言**: Go 1.25.5
- **框架**: [Gozero](https://go-zero.dev/) v1.9.3（微服务框架）
- **数据库**: MySQL 8.x
- **缓存**: Redis
- **架构**: 微服务 RPC 架构

### 前端技术栈

- **框架**: [Art Design Pro](https://www.artd.pro/docs/zh/)（企业级 UI 框架）

### 服务架构

```
kube-nova/
├── application/              # 微服务应用
│   ├── portal-api           # 门户 API（用户/权限/告警）
│   ├── portal-rpc           # 门户 RPC 服务
│   ├── manager-api          # 管理 API（集群/项目）
│   ├── manager-rpc          # 管理 RPC 服务
│   ├── console-api          # 控制台 API
│   ├── console-rpc          # 控制台 RPC 服务
│   └── workload-api         # 工作负载 API
├── common/                   # 公共代码
│   ├── handler              # HTTP 处理器
│   ├── interceptors         # RPC 拦截器
│   ├── k8smanager          # K8s 客户端管理
│   ├── middleware          # 中间件
│   ├── prometheusmanager   # Prometheus 客户端
│   ├── utils               # 工具函数
│   └── vars                # 全局变量
├── pkg/                      # 第三方包封装
│   ├── casbinadapter        # Casbin 适配器
│   ├── jwt                  # JWT 认证
│   ├── storage              # 对象存储
│   └── utils                # 通用工具
├── manifests/                # K8s 部署清单
├── dockerfile/               # 容器镜像构建
├── scripts/                  # 脚本工具
└── sql/                      # 数据库脚本
```

------

## 🚀 快速开始

### 前置要求

- Kubernetes 1.21+
- MySQL 8.0+
- Redis 7.0+
- Go 1.25.5+（仅开发环境）

### 📚 部署文档

- **部署文档**: [部署指南](https://www.cnblogs.com/yanshicheng/articles/19363328)

### 🐳 Docker 镜像

| 服务模块               | Docker Hub                           | 阿里云镜像                                                                   |
|--------------------|--------------------------------------|-------------------------------------------------------------------------|
| portal-api         | `ikubeops/portal-api:latest`         | `registry.cn-hangzhou.aliyuncs.com/kube-nova/portal-api:latest`         |
| portal-rpc         | `ikubeops/portal-rpc:latest`         | `registry.cn-hangzhou.aliyuncs.com/kube-nova/portal-rpc:latest`         |
| console-api        | `ikubeops/console-api:latest`        | `registry.cn-hangzhou.aliyuncs.com/kube-nova/console-api:latest`        |
| console-rpc        | `ikubeops/console-rpc:latest`        | `registry.cn-hangzhou.aliyuncs.com/kube-nova/console-rpc:latest`        |
| manager-api        | `ikubeops/manager-api:latest`        | `registry.cn-hangzhou.aliyuncs.com/kube-nova/manager-api:latest`        |
| manager-rpc        | `ikubeops/manager-rpc:latest`        | `registry.cn-hangzhou.aliyuncs.com/kube-nova/manager-rpc:latest`        |
| workload-api       | `ikubeops/workload-api:latest`       | `registry.cn-hangzhou.aliyuncs.com/kube-nova/workload-api:latest`       |
| kube-nova-web      | `ikubeops/kube-nova-web:latest`      | `registry.cn-hangzhou.aliyuncs.com/kube-nova/kube-nova-web:latest`      |
| kube-nova-operator | `ikubeops/kube-nova-operator:latest` | `registry.cn-hangzhou.aliyuncs.com/kube-nova/kube-nova-operator:latest` |
------

## 💡 功能特性

### 一、核心平台能力

**1. 多租户管理**

- 项目 → 集群配额 → 工作空间 三级隔离
- 支持资源超分（CPU/内存/GPU）
- 完整的 RBAC 权限体系

**2. 多集群管理**

- 支持多集群接入（Kubeconfig/Token/证书）
- 集群资源监控和配额管理
- 节点标签、污点、调度控制

**3. 告警中心**

- 8 种通知渠道（钉钉/企微/飞书/邮件/短信/语音/Webhook/站内信）
- 告警分组、分级路由
- 实时消息推送（WebSocket）

### 二、应用管理

**4. 工作负载全生命周期**

- 支持 Deployment/StatefulSet/DaemonSet/Job/CronJob
- 启停、重启、扩缩容、滚动更新
- 版本历史和一键回滚
- 镜像批量更新

**5. 核心资源管理**

- ConfigMap/Secret 配置管理
- Service/Ingress 流量管理
- PVC 存储管理
- ServiceAccount/RBAC 权限

**6. 应用编排增强**

- 环境变量注入（多种来源）
- 健康检查（Liveness/Readiness/Startup）
- 调度策略（亲和性/反亲和性/容忍度）
- 存储卷配置

### 三、自动化能力

**7. 弹性伸缩**

- HPA：基于多种指标（CPU/内存/自定义）
- VPA：自动资源推荐和调整

**8. 灰度发布（Flagger）**

- 金丝雀发布流程控制
- 指标分析和自动回滚
- 流量权重动态调整

### 四、运维工具

**9. 容器镜像仓库（Harbor）**

- 多仓库统一管理
- 项目级权限隔离
- GC、保留策略、镜像复制

**10. Pod 运维**

- 日志查看（实时流/历史/下载）
- 交互式终端（Exec）
- 文件管理（浏览/编辑/上传/下载/压缩）

**11. 监控体系（5 个维度）**

- **Pod 监控**: 资源、网络、存储、健康状态
- **Node 监控**: CPU/内存/磁盘/网络/系统指标
- **Namespace 监控**: 配额、工作负载、Top 排行
- **集群监控**: 控制面（API/etcd/调度器/控制器）、资源容量
- **Ingress 监控**: 流量、性能、错误率、证书

**12. 审计日志**

- 四级审计（集群/项目/工作空间/应用）
- 多维度查询和过滤
- 操作记录追溯

------

## 🌟 平台特色

| 特性                  | 说明                      |
| --------------------- | ------------------------- |
| ✅ **多租户隔离**      | 三级隔离 + 资源超卖       |
| ✅ **统一告警**        | 8 种渠道 + 智能路由       |
| ✅ **镜像全生命周期**  | 多仓库 + 策略管理         |
| ✅ **完整的 Pod 运维** | 日志 + 终端 + 文件管理    |
| ✅ **五维监控**        | 覆盖 Pod 到集群的完整视图 |
| ✅ **灰度发布**        | 自动化金丝雀部署          |
| ✅ **审计合规**        | 全链路操作追溯            |

------

## 📸 平台截图

<details> <summary>点击查看更多截图</summary>

### 登录页面

![登录页面](https://images.ikubeops.com/kube-nova/login.png)

### 集群管理

![集群管理](https://images.ikubeops.com/kube-nova/cluster-manager.png) ![集群节点](https://images.ikubeops.com/kube-nova/cluster-node.png) ![集群监控](https://images.ikubeops.com/kube-nova/cluster-monitor.png) ![集群中间件](https://images.ikubeops.com/kube-nova/cluster-m.png)

### 项目管理

![项目资源池](https://images.ikubeops.com/kube-nova/project-resource.png) ![项目工作空间](https://images.ikubeops.com/kube-nova/project-workspace.png)

### 应用管理

![应用详情](https://images.ikubeops.com/kube-nova/app-info.png) ![服务版本](https://images.ikubeops.com/kube-nova/app-version.png)

### Pod 运维

![Pod 日志管理](https://images.ikubeops.com/kube-nova/pod-log.png) ![Pod 终端](https://images.ikubeops.com/kube-nova/pod-terminal.png) ![Pod 文件管理](https://images.ikubeops.com/kube-nova/pod-file-manager.png)

### 监控体系

![Namespace 监控](https://images.ikubeops.com/kube-nova/namespace-monitor.png) ![Pod 监控](https://images.ikubeops.com/kube-nova/pod-monirot.png)

### 告警中心

![告警渠道](https://images.ikubeops.com/kube-nova/monitor-channel.png) ![钉钉告警](https://images.ikubeops.com/kube-nova/dingding-notifcation.png) ![飞书告警](https://images.ikubeops.com/kube-nova/feishu-notification.png) ![邮件告警](https://images.ikubeops.com/kube-nova/email-notification.png)

</details>

------

## 🤝 贡献与加入

我们欢迎任何形式的贡献！无论是报告 Bug、提出新功能建议，还是直接提交代码，我们都非常感谢。

### 如何贡献

1. Fork 本仓库
2. 创建你的特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交你的改动 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 提交 Pull Request

### 加入我们

如果你对 Kubernetes、云原生、微服务架构感兴趣，欢迎加入我们一起完善这个项目！

**项目维护者**

- **YanShicheng** - [ikubeops@gmail.com](mailto:ikubeops@gmail.com)

------

## 📄 开源协议

本项目采用 [Apache License 2.0](https://www.apache.org/licenses/LICENSE-2.0) 协议开源。

------

## 🔗 相关链接

- 🌐 [在线演示](https://www.ikubeops.com/)
- 📖 [Gozero 框架](https://go-zero.dev/)
- 🎨 [Art Design Pro](https://www.artd.pro/docs/zh/)

------

**如果这个项目对你有帮助，请给个 ⭐ Star 支持一下！**

**期待你的贡献和反馈 🎉**