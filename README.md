<p align="center">
  <b>🚀 新一代企业级 Kubernetes 多集群管理平台</b>
</p>

<p align="center">
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/Go-1.25.5-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://go-zero.dev/"><img src="https://img.shields.io/badge/Go--Zero-v1.9.4-1E88E5" alt="Go-Zero"></a>
  <a href="https://kubernetes.io/"><img src="https://img.shields.io/badge/Kubernetes-1.21+-326CE5?logo=kubernetes&logoColor=white" alt="Kubernetes"></a>
  <a href="https://vuejs.org/"><img src="https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js&logoColor=white" alt="Vue"></a>
  <br>
  <a href="https://www.mysql.com/"><img src="https://img.shields.io/badge/MySQL-8.0+-4479A1?logo=mysql&logoColor=white" alt="MySQL"></a>
  <a href="https://redis.io/"><img src="https://img.shields.io/badge/Redis-7.0+-DC382D?logo=redis&logoColor=white" alt="Redis"></a>
  <a href="https://grpc.io/"><img src="https://img.shields.io/badge/gRPC-Protocol-244c5a?logo=grpc" alt="gRPC"></a>
  <a href="https://github.com/yanshicheng/kube-nova"><img src="https://img.shields.io/github/stars/yanshicheng/kube-nova?style=social" alt="GitHub Stars"></a>
</p>

<p align="center">
  <a href="https://kube-nova.ikubeops.com">🌐 在线演示</a> | 
  <a href="https://www.ikubeops.com">📖 文档中心</a> | 
  <a href="https://wiki-images.yanshicheng.com/common/kube-nova-wechat.png">💬 加入社区</a>
</p>

---

## 🎯 什么是 Kube-Nova？

**Kube-Nova** 是一个生产就绪的企业级 Kubernetes 多集群管理平台，重新定义了组织如何大规模管理容器化工作负载。采用前沿的微服务架构，基于 Go-Zero 框架构建，提供无与伦比的性能、灵活性和卓越的运维能力。

### 💡 为什么选择 Kube-Nova

传统的 Kubernetes 管理平台在企业场景中存在诸多不足。Kube-Nova 通过创新解决方案填补了这些关键空白：

| 🏗️ 革命性架构                                                 | ⚡ 企业级运维                                                 |
| :----------------------------------------------------------- | :----------------------------------------------------------- |
| **三层多租户隔离**：项目 → 集群配额 → 工作空间，支持资源超分 | **五维监控体系**：Pod、节点、命名空间、集群、Ingress 全方位指标 |
| **微服务卓越性**：7 个专业化服务，gRPC 高性能通信            | **智能告警系统**：6 种通知渠道，智能路由和聚合               |
| **事件驱动同步**：增量监听系统，Leader 选举实现实时集群状态管理 | **高级 Pod 操作**：分片文件上传、实时日志、交互式终端        |

### 🚀 即将上线

* **DevOps CI/CD 平台**：集成持续集成和部署流水线
* **Istio 服务网格**：高级流量管理、可观测性和安全性

---

## ✨ 核心功能

### 🏢 多租户与资源管理

**三层隔离架构**

```text
项目（租户）
 ├─ 集群配额（资源池）
 │   ├─ CPU/内存/GPU 超分配置
 │   └─ 跨集群资源分配
 └─ 工作空间（命名空间组）
     ├─ 细粒度 RBAC
     └─ 资源配额强制执行
```

**核心能力**

* ✅ 可配置的资源超分比例
* ✅ 分层权限继承
* ✅ 自动成本分配和计费
* ✅ 多平台动态菜单管理
* ✅ 基于部门的组织架构

### 🌐 统一多集群管理

* **灵活认证方式**：支持 Kubeconfig、Token 和证书方式接入集群
* **集群元数据**：环境标签（dev/test/staging/prod）、云厂商追踪、区域/可用区管理
* **实时监控**：控制平面健康状态（API Server、etcd、Scheduler、Controller Manager）
* **节点操作**：标签、污点、隔离和高级调度控制
* **网络发现**：自动集群网络拓扑可视化

### 🔔 智能告警系统

| 渠道类型 | 支持平台 | 高级特性 |
| --- | --- | --- |
| **即时通讯** |  | • 基于严重级别的路由（信息/警告/严重）<br>

<br>• 告警分组和聚合 |
| **通知推送** |  | • 实时 WebSocket 推送<br>

<br>• 值班排班管理<br>

<br>• 通知历史追踪 |
| **开发中** |  |  |

### 📦 容器镜像仓库管理（Harbor 深度集成）

| 核心功能 | 高级策略 |
| --- | --- |
| • **多仓库统一管理**（Harbor、Docker Registry、Nexus）<br>

<br>• **三层权限控制**（仓库 → 项目 → 镜像库）<br>

<br>• **项目级配额管理**<br>

<br>• **全局/项目镜像搜索**<br>

<br>• **成员角色管理** | • **垃圾回收**：定时/手动 GC<br>

<br>• **保留策略**：基于模板规则（最新K个/N天未拉取）<br>

<br>• **复制策略**：跨仓库复制，支持过滤器<br>

<br>• **用户管理**：Harbor 用户与角色分配 |

### 🚀 工作负载生命周期管理

* **完整工作负载支持**：Deployment, StatefulSet, DaemonSet, Job, CronJob
* **高级操作**：启动/停止、重启、扩缩容、滚动更新、一键回滚
* **批量镜像更新**：同时更新多个工作负载的镜像
* **版本历史**：追踪并回滚到之前的配置，支持 Diff 对比
* **YAML 优先**：表单与 YAML 无缝转换

### 🐳 高级 Pod 操作

| 日志管理 | 文件操作 | 交互式终端 |
| --- | --- | --- |
| • 实时流式日志<br>

<br>• 历史日志查看<br>

<br>• 多容器支持<br>

<br>• 日志下载<br>

<br>• 高级过滤 | • 浏览容器文件系统<br>

<br>• 在线编辑文件<br>

<br>• **分片文件上传**（大文件）<br>

<br>• 文件下载<br>

<br>• 压缩/解压归档 | • Web 终端访问<br>

<br>• 多容器选择<br>

<br>• 终端大小调整<br>

<br>• 会话持久化<br>

<br>• 复制/粘贴支持 |

### 📊 五维监控体系

```text
┌─────────────────────────────────────────────────────────────────┐
│  Pod 级别      → CPU、内存、网络 I/O、磁盘 I/O、重启次数          │
│  节点级别      → 系统指标、资源利用率、Kubelet 状态               │
│  命名空间级别  → 配额使用、工作负载分布、Top 资源消耗             │
│  集群级别      → 控制平面健康、资源容量、API 延迟                 │
│  Ingress 级别  → 流量、响应时间、错误率、SSL 证书                 │
└─────────────────────────────────────────────────────────────────┘

```

### 🎯 自动化与安全

* **金丝雀部署 (Flagger)**：渐进式交付、基于指标的自动回滚、蓝绿部署。
* **自动扩缩容**：HPA（水平）、VPA（垂直）、自定义指标支持。
* **全面审计日志**：四级审计（集群/项目/空间/应用）、操作追踪、Diff 历史。
* **网络策略管理**：可视化策略编辑、受影响 Pod 分析、实时验证。
* **成本管理**：基于资源的计费、多维度定价、预算告警。

---

## 🏗️ 架构亮点

### 微服务架构

Kube-Nova 通过 7 个专业化微服务实现清晰的关注点分离：

| 架构层级 | 服务组件 | 职责描述 |
| --- | --- | --- |
| **前端** | Vue 3 + Art Design | 用户交互界面 |
| **接入层** | **Portal-API/RPC** | 用户认证、RBAC、告警管理、站内消息 |
| **管理层** | **Manager-API/RPC** | 集群/项目管理、计费、审计日志、资源同步 |
| **控制层** | **Console-API/RPC** | 监控仪表板、Pod 操作、镜像仓库管理 |
| **工作负载** | **Workload-API** | 工作负载 CRUD、扩缩容、更新、回滚、金丝雀部署 |

### 关键架构创新

1. **增量监听系统**：分布式资源变更追踪，Leader 选举实现高可用。
2. **事件驱动同步**：Kubernetes 监听事件触发实时数据库更新。
3. **多 Prometheus 支持**：每个集群可拥有独立的 Prometheus 实例，统一查询。
4. **gRPC 内部通信**：低延迟、类型安全的微服务间通信。

---

## 🛠️ 技术栈

| 领域 | 核心技术 |
| --- | --- |
| **后端** | Go 1.25.5, Go-Zero 1.9.4, gRPC, MySQL 8.0+, Redis 7.0+ |
| **前端** | Vue 3.x, Art Design Pro, Pinia, Axios, WebSocket |
| **基础设施** | Kubernetes 1.21+, Docker, Harbor 2.0+, GitLab CI/GitHub Actions |
| **关键依赖** | Client-go, Prometheus, JWT + Casbin, MinIO, Istio (可选) |

---

## 🚀 快速开始

### 前置要求

* Kubernetes 集群 1.21+
* MySQL 8.0+
* Redis 7.0+
* Go 1.25.5+（开发用）

### 安装方式

#### 1. Kubernetes 部署（推荐）

```bash
# 克隆仓库
git clone [https://github.com/yanshicheng/kube-nova.git](https://github.com/yanshicheng/kube-nova.git)
cd kube-nova

# 应用 Kubernetes 清单
kubectl apply -f manifests/

# 检查部署状态
kubectl get pods -n kube-nova

# 访问平台 (端口转发)
kubectl port-forward -n kube-nova svc/kube-nova-web 8080:80

```

#### 2. Docker Compose

```bash
git clone [https://github.com/yanshicheng/kube-nova.git](https://github.com/yanshicheng/kube-nova.git)
cd kube-nova
docker-compose up -d
# 访问 http://localhost:8080

```

#### 3. 本地开发

```bash
# 初始化数据库
mysql -u root -p < sql/kube_nova.sql

# 启动各个服务 (示例)
cd application/portal-api && go run portal.go &
# ... 重复启动其他服务

```

### 访问平台

* **URL**：`http://your-domain:8080`
* **默认凭证**：`admin` / `admin123` （请首次登录后修改）

---

## 📁 项目结构

```text
kube-nova/
├── application/           # 微服务应用 (API & RPC)
│   ├── portal-api/        # 认证与基础服务
│   ├── manager-api/       # 集群与项目管理
│   ├── console-api/       # 控制台与监控
│   └── workload-api/      # 工作负载管理
├── common/                # 共享库 (K8s Client, Utils, Middleware)
├── pkg/                   # 第三方封装 (Casbin, JWT, MinIO)
├── manifests/             # K8s 部署清单
├── dockerfile/            # Docker 构建文件
└── sql/                   # 数据库脚本

```

---

## 📸 平台截图

<details>
<summary><b>🖱️ 点击展开查看详细截图</b></summary>

| 仪表板与集群管理 |  |
| --- | --- |
| 

<br><b>登录页面</b> | 

<br><b>集群管理</b> |
| 

<br><b>集群节点</b> | 

<br><b>集群监控</b> |

| 项目与应用管理 |  |
| --- | --- |
| 

<br><b>项目资源</b> | 

<br><b>项目工作空间</b> |
| 

<br><b>应用详情</b> | 

<br><b>服务版本</b> |

| Pod 操作与监控 |  |
| --- | --- |
| 

<br><b>Pod 日志</b> | 

<br><b>Pod 终端</b> |
| 

<br><b>文件管理</b> | 

<br><b>钉钉告警</b> |

</details>

---

## 🤝 贡献指南

1. **Fork** 本仓库
2. **创建分支**: `git checkout -b feature/amazing-feature`
3. **提交更改**: `git commit -m "feat: add amazing feature"`
4. **推送**: `git push origin feature/amazing-feature`
5. **提交 PR**: 确保所有测试通过，并遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范。

---

## 📦 代码仓库

| 组件 | GitHub | Gitee |
| --- | --- | --- |
| **后端** | [yanshicheng/kube-nova](https://github.com/yanshicheng/kube-nova) | [ikubeops/kube-nova](https://gitee.com/ikubeops/kube-nova) |
| **前端** | [yanshicheng/kube-nova-web](https://github.com/yanshicheng/kube-nova-web) | [ikubeops/kube-nova-web](https://gitee.com/ikubeops/kube-nova-web) |
| **Operator** | [yanshicheng/kube-nova-operator](https://github.com/yanshicheng/kube-nova-operator) | [ikubeops/kube-nova-operator](https://gitee.com/ikubeops/kube-nova-operator) |

---

## 📄 开源协议

本项目采用 [Apache License 2.0](https://www.google.com/search?q=LICENSE) 开源协议。

Copyright 2025 YanShicheng

---

## 🌟 Star 历史

---

<p align="center">
<b>用 ❤️ 由 Kube-Nova 团队构建</b>




<a href="#kube-nova">⬆ 回到顶部</a>
</p>

```

```