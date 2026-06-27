# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 1. 通用沟通规则

- 默认使用中文回复。
- 不使用 emoji、图标或花哨排版。
- 中文注释，注释保证简洁准确，不能出现 emoji、图标。
- 输出风格保持简洁、专业、直接。
- 不写空话，不重复用户问题，不做无意义铺垫。
- 修改代码前先阅读相关文件，再进行变更。
- 仅做当前任务要求的修改，不做顺手重构，不扩散改动范围。
- 涉及接口、模型、配置、生成代码时，先确认源文件位置，再修改，不凭猜测直接改产物。

## 2. 项目概览

这是 Kube-Nova 的 Go 后端仓库，基于 Go-Zero，主要用于 Kubernetes 多集群管理。

前端不在本仓库内。当前仓库核心代码集中在 `application/` 目录，共 10 个应用：

**核心平台服务（7个）**：
- `application/portal-api`：gozero API 服务，主要负责门户、认证、权限、告警、站内消息等能力。
- `application/portal-rpc`：gozero RPC 服务，主要负责门户、认证、权限、告警、站内消息等后端实现。
- `application/manager-api`：gozero API 服务，主要管理 Kubernetes、项目、同步、监控等能力。
- `application/manager-rpc`：gozero RPC 服务，主要管理 Kubernetes、项目、同步、账单等能力。
- `application/console-api`：gozero API 服务，主要负责控制台、监控仪表板、镜像仓库管理。
- `application/console-rpc`：gozero RPC 服务，主要负责镜像仓库、仓库项目绑定、定时任务等。
- `application/workload-api`：gozero API 服务，主要负责工作负载管理。

**DevOps 平台服务（3个）**：
- `application/devops-api`：gozero API 服务，DevOps 平台入口。
- `application/devops-manager-rpc`：gozero RPC 服务，DevOps 管理能力。
- `application/devops-pipeline-rpc`：gozero RPC 服务，CI/CD 流水线管理。
- `application/devops-quality-rpc`：gozero RPC 服务，代码质量管理。

整体职责边界：

- API 服务：对外提供 HTTP 接口，负责参数接收、鉴权、中间件接入、调用下游 RPC。
- RPC 服务：承载核心业务逻辑、数据库访问、模型层、后台任务。
- `workload-api` 没有对应的 workload-rpc，它主要通过已有 RPC 服务和公共 Kubernetes 能力完成业务。

## 3. 代码组织规则

### 3.1 API 服务组织方式

API 服务统一遵循 Go-Zero API 结构：

- 应用入口：`application/<app-name>/*.go`
- API 定义目录：`application/<app-name>/api/*.api`
- API 聚合入口：`application/<app-name>/<app-name>.api`
- handler：`application/<app-name>/internal/handler`
- logic：`application/<app-name>/internal/logic`
- svc：`application/<app-name>/internal/svc`
- 配置：`application/<app-name>/etc/*.yaml`

重要约定：

- 每个 API 服务的 `api/*.api` 是该应用分模块定义的接口文件。
- `application/<app-name>/<app-name>.api` 负责统一 `import` 这些分模块 `.api` 文件。
- 修改 API 接口时，应优先修改 `api/*.api` 或聚合 `.api`，再执行对应的生成命令。
- API 服务通常不直接承载核心数据逻辑，更多是参数处理、鉴权、调用 RPC、拼装响应。

例如：

- `application/portal-api/portal.api` 引用 `application/portal-api/api/*.api`
- `application/manager-api/manager.api` 引用 `application/manager-api/api/*.api`
- `application/console-api/console.api` 引用 `application/console-api/api/*.api`
- `application/workload-api/workload.api` 引用 `application/workload-api/api/*.api`
- `application/devops-api/devops.api` 引用 `application/devops-api/api/*.api`

### 3.2 RPC 服务组织方式

RPC 服务统一遵循 Go-Zero RPC 结构：

- 应用入口：`application/<rpc-name>/*.go`
- Proto 定义：`application/<rpc-name>/<rpc-name>.proto`
- server：`application/<rpc-name>/internal/server`
- logic：`application/<rpc-name>/internal/logic`
- svc：`application/<rpc-name>/internal/svc`
- 配置：`application/<rpc-name>/etc/*.yaml`
- 生成产物：`application/<rpc-name>/pb`、`application/<rpc-name>/client`

重要约定：

- RPC 接口定义统一在 `application/<rpc-name>/<rpc-name>.proto`。
- 修改 RPC 接口时，先改 `.proto`，再执行对应生成命令。
- `pb/`、`client/`、部分 `routes.go` 等内容通常属于生成代码，修改前先确认是否应该改源文件而不是直接改生成产物。
- RPC 服务的 `internal/svc` 往往是依赖注入核心位置，通常会初始化 DB、Redis、model、K8s manager、Prometheus manager、定时任务等。

### 3.3 数据库访问边界

数据库连接和模型层只允许放在 RPC 服务中，不要在 API 服务里直接连数据库。

模型代码位置：

- `application/portal-rpc/internal/model`
- `application/manager-rpc/internal/model`
- `application/console-rpc/internal/model`
- `application/devops-manager-rpc/internal/model`
- `application/devops-pipeline-rpc/internal/model`
- `application/devops-quality-rpc/internal/model`

补充说明：

- `console-rpc` 的模型主要在 `application/console-rpc/internal/model/repository`
- API 服务应通过 RPC 调用获取数据，不应新增本地 DB model 或直接写 SQL
- 如果涉及表结构、缓存模型、数据库事务，优先去对应 RPC 服务中查找实现
- 仓库里存在大量 `*_gen.go`，这类文件通常是 model 生成产物，业务修改优先看非 `_gen.go` 文件

## 4. 重要架构认知

### 4.1 服务调用关系

整体上是典型的 API + RPC 分层：

**核心平台**：
- `portal-api` 主要调用 `portal-rpc`
- `manager-api` 主要调用 `manager-rpc`，也会调用 `portal-rpc`
- `console-api` 会调用 `console-rpc`、`manager-rpc`、`portal-rpc`
- `workload-api` 主要通过 `manager-rpc`、`portal-rpc` 以及公共 Kubernetes 能力完成业务

**DevOps 平台**：
- `devops-api` 主要调用 `devops-manager-rpc`、`devops-pipeline-rpc`、`devops-quality-rpc`
- DevOps 服务也会调用核心平台的 `portal-rpc`、`manager-rpc` 获取基础数据

因此排查一个接口时，通常要同时看：

1. API 层 handler / logic
2. `internal/svc` 中注入了哪些 RPC client / middleware
3. 对应 RPC 服务的 server / logic / model

### 4.2 公共能力目录

跨服务复用能力主要集中在：

- `common/`：公共中间件、拦截器、错误处理、Kubernetes 管理、Prometheus 管理、校验器等
- `pkg/`：更底层的工具封装，如 JWT、Casbin、MinIO 存储等
- `template/`：Go-Zero 自定义代码生成模板

其中几个目录尤其重要：

- `common/interceptors`：RPC 请求元数据、错误拦截处理
- `common/middleware`：HTTP 中间件，如 JWT、panic recovery
- `common/k8smanager`：Kubernetes 访问与资源操作的公共封装
- `common/prometheusmanager`：Prometheus 访问与监控查询封装
- `common/logmanager`：日志管理（Loki/Elasticsearch 集成）
- `common/devops`：DevOps 平台公共能力
- `common/utils`：通用工具函数
- `common/verify`：参数校验器

如果需求涉及集群、节点、Pod、监控、日志、资源对象，不要只看 API 层，通常还需要看 `common/k8smanager` 或 `common/prometheusmanager`。

### 4.3 各 RPC 服务的职责重点

**核心平台**：
- `portal-rpc`：偏用户、角色、菜单、权限、登录、站内消息、告警通知。
- `manager-rpc`：偏集群管理、项目管理、资源同步、账单、告警消费。
- `console-rpc`：偏镜像仓库、仓库项目绑定、定时任务等。

**DevOps 平台**：
- `devops-manager-rpc`：DevOps 项目管理、应用管理、环境管理。
- `devops-pipeline-rpc`：CI/CD 流水线、构建任务、部署任务。
- `devops-quality-rpc`：代码质量检测、测试覆盖率、代码扫描。

### 4.4 workload-api 的特点

`workload-api` 是工作负载管理入口，但它本身不是数据库核心服务。

它更像是：

- 对 Kubernetes 工作负载操作的 API 聚合层
- 通过 `manager-rpc` 获取集群信息、权限信息、基础数据
- 通过公共 `k8smanager` 对集群资源执行实际操作

所以修改 `workload-api` 时，经常需要联动查看：

- `application/workload-api/internal/*`
- `common/k8smanager/*`
- `application/manager-rpc/*`

### 4.5 配置与依赖注入规律

每个服务通常都有：

- 入口文件：加载 `etc/*.yaml`
- `internal/config`：配置结构定义
- `internal/svc`：依赖注入

理解某个服务最快的入口顺序通常是：

1. 先看 `application/<service>/*.go`
2. 再看 `application/<service>/internal/config`
3. 再看 `application/<service>/internal/svc`
4. 最后再进入 handler / logic / server / model

## 5. 常用开发命令

### 5.1 基础检查命令

在仓库根目录执行：

- 格式化：`make fmt`
- 静态检查：`make vet`
- Lint：`make lint`
- 运行测试：`make test`
- 生成覆盖率：`make test-coverage`
- 更新依赖：`make tidy`
- 查看可用命令：`make help`

### 5.2 单测命令

运行单个包：

```bash
go test ./application/portal-rpc/internal/notification
```

运行单个测试：

```bash
go test ./application/portal-rpc/internal/notification -run TestFeishu -v
```

通用写法：

```bash
go test ./path/to/pkg -run TestName -v
```

### 5.3 启动服务

统一在仓库根目录执行。

**核心平台服务**：
- 启动 portal-api：`make run-portal-api`
- 启动 portal-rpc：`make run-portal-rpc`
- 启动 manager-api：`make run-manager-api`
- 启动 manager-rpc：`make run-manager-rpc`
- 启动 console-api：`make run-console-api`
- 启动 console-rpc：`make run-console-rpc`
- 启动 workload-api：`make run-workload-api`

**DevOps 平台服务**：
- 启动 devops-api：`make run-devops-api`
- 启动 devops-manager-rpc：`make run-devops-manager-rpc`
- 启动 devops-pipeline-rpc：`make run-devops-pipeline-rpc`
- 启动 devops-quality-rpc：`make run-devops-quality-rpc`

也可以直接使用 `go run`：

```bash
go run ./application/portal-api/portal.go -f ./application/portal-api/etc/portal-api.yaml
go run ./application/portal-rpc/portal.go -f ./application/portal-rpc/etc/portal.yaml
go run ./application/manager-api/manager.go -f ./application/manager-api/etc/manager-api.yaml
go run ./application/manager-rpc/manager.go -f ./application/manager-rpc/etc/manager.yaml
go run ./application/console-api/console.go -f ./application/console-api/etc/console-api.yaml
go run ./application/console-rpc/console.go -f ./application/console-rpc/etc/console.yaml
go run ./application/workload-api/workload.go -f ./application/workload-api/etc/workload-api.yaml
go run ./application/devops-api/devops.go -f ./application/devops-api/etc/devops-api.yaml
go run ./application/devops-manager-rpc/devops-manager.go -f ./application/devops-manager-rpc/etc/devops-manager.yaml
go run ./application/devops-pipeline-rpc/devops-pipeline.go -f ./application/devops-pipeline-rpc/etc/devops-pipeline.yaml
go run ./application/devops-quality-rpc/devops-quality.go -f ./application/devops-quality-rpc/etc/devops-quality.yaml
```

### 5.4 生成代码

通过根目录 `Makefile` 生成，不要自己临时拼 `goctl` 命令。

**核心平台服务**：
- 生成 portal-api：`make gen-portal-api`
- 生成 portal-rpc：`make gen-portal-rpc`
- 生成 manager-api：`make gen-manager-api`
- 生成 manager-rpc：`make gen-manager-rpc`
- 生成 console-api：`make gen-console-api`
- 生成 console-rpc：`make gen-console-rpc`
- 生成 workload-api：`make gen-workload-api`

**DevOps 平台服务**：
- 生成 devops-api：`make gen-devops-api`
- 生成 devops-manager-rpc：`make gen-devops-manager-rpc`
- 生成 devops-pipeline-rpc：`make gen-devops-pipeline-rpc`
- 生成 devops-quality-rpc：`make gen-devops-quality-rpc`

规则：

- 改 `.api` 后，执行对应 `make gen-*-api`
- 改 `.proto` 后，执行对应 `make gen-*-rpc`
- 本仓库使用 `template/` 下的自定义模板，生成结果依赖这些模板

## 6. 明确约束

- 根目录 `Makefile` 不允许修改。
- 优先使用已有 `make run-*`、`make gen-*`、`make fmt`、`make test` 等命令。
- 能通过现有 `make` 命令完成的事情，不要引入新的生成方式。
- 不要在 API 服务新增数据库访问逻辑。
- 不要跳过 `.api` / `.proto` 源文件而直接修改生成产物。
- 修改代码时优先遵循当前服务已有结构和命名，不要为了"统一风格"做无关重命名。
- 根目录 `Makefile` 中虽然有通用 `build` / `run` 目标，但该仓库实际按 `application/*` 多服务组织，日常开发优先使用各服务的 `run-*` / `gen-*` 目标。

## 7. 容易踩坑的点

- README 中本地开发示例里的 SQL 文件名写的是 `sql/kube_nova.sql`，但仓库实际存在的是 `sql/db.sql`。
- 仓库启用了 `.husky/pre-commit`，禁止直接在 `main` 或 `master` 分支提交。
- 本仓库有大量生成文件，例如：
  - `application/*/internal/handler/routes.go`
  - `application/*/pb/*`
  - `application/*/client/*`
  - `application/*/internal/model/*_gen.go`
- 修改生成文件前，先确认是否应该改源文件：`.api`、`.proto`、模板文件或非 `_gen.go` 业务文件。
- DevOps 平台服务是新增模块，与核心平台服务有依赖关系，修改时需要注意服务间调用。

## 8. 运行与环境前提

根据 README，这个仓库常见开发前提为：

- Go 1.25.5+
- MySQL 8.0+
- Redis 7.0+
- Kubernetes 1.21+

技术栈重点：

- Go-Zero 1.9.4
- gRPC
- MySQL
- Redis
- client-go
- Prometheus
- JWT + Casbin
- MinIO
- Harbor 2.0+

## 9. 阅读代码的优先路径

当需要快速理解一个功能时，建议按下面顺序看：

1. 先看对应应用入口：`application/<app-name>/*.go`
2. 再看 `internal/svc`，确认注入了哪些依赖
3. 再看 `internal/handler` / `internal/logic`
4. 如果是 API 服务，再追到其调用的 RPC client
5. 再进入对应 RPC 服务的 `internal/server` / `internal/logic` / `internal/model`
6. 如果涉及 K8s 或监控，再看 `common/k8smanager` / `common/prometheusmanager`
7. 如果涉及 DevOps 功能，再看 `common/devops`

这样比只在单个目录里搜索更快。

## 10. 项目特色功能

### 10.1 三层多租户架构

项目实现了 项目（租户）→ 集群配额（资源池）→ 工作空间（命名空间组）的三层隔离，支持资源超分配置。

### 10.2 增量监听系统

使用分布式资源变更追踪，Leader 选举实现高可用的事件驱动同步机制。

### 10.3 五维监控体系

支持 Pod、节点、命名空间、集群、Ingress 五个维度的监控指标采集和展示。

### 10.4 Harbor 深度集成

支持多仓库统一管理、三层权限控制、垃圾回收、保留策略、复制策略等高级功能。

### 10.5 高级 Pod 操作

支持实时日志、分片文件上传、Web 终端、文件管理等高级操作。
