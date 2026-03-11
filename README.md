# Kube-Sentinel

Kube-Sentinel 是一个面向 Kubernetes 夜间值班场景的轻量哨兵工具。

当前首发版本聚焦在一个明确收敛的能力边界上：

- 通过 Alertmanager Webhook 接收故障事件
- 将事件映射为 `HealingRequest`
- 对 Deployment 执行一次安全优先的 L1 最小动作尝试
- 在任何自动写操作前执行门禁判定与快照校验
- 输出结构化审计记录、运行时事件和指标，并为 Agent/Headlamp/Grafana/kubectl 提供稳定关联键
- 通过 Agent v1 输出固定五段式分诊结果，并生成 Telegram incident card

它不是完整可观测平台，也不是自治运维平台。当前版本的目标，是交付一个可快速部署、可解释、可人工接管的夜间值班哨兵。

## 当前支持范围

首发版本已经完成的能力：

- `Deployment` 单次 L1 自动处置闭环
- Alertmanager Webhook 接入与事件幂等去重
- `HealingRequest` 默认值、校验约束和状态语义
- L1 写动作前的安全门禁：维护窗口、速率限制、爆炸半径、熔断
- 写前快照创建失败即阻断
- K8s Event、结构化审计和 Prometheus 指标输出
- Agent 所需的 incident summary / recommendation / handoff 状态语义
- 质量门禁：`go test`、`race`、`vet`、`golangci-lint`、CRD/Helm 一致性检查

当前明确不属于首发范围的能力：

- `StatefulSet` 自动写动作
- `Deployment` L2/L3 自动化恢复
- 复杂发布门禁自动化
- 完整生产级多集群放量编排
- 额外的自定义 UI 查询后端或独立控制台读模型
- Agent 驱动的生产写动作

## 核心链路

```text
Alertmanager Webhook
        |
        v
Receiver (/alertmanager/webhook)
        |
        v
HealingRequest
        |
        v
Reconciler
        |
        v
Orchestrator
        |
        +--> 安全门禁
        |     - maintenance window
        |     - rate limit
        |     - blast radius
        |     - circuit breaker
        |
        +--> 持久快照
        |
        +--> Deployment L1 动作
        |
        +--> Agent 摘要 / 建议 / 交接
        |
        +--> 人工介入建议
        |
        +--> Audit / Event / Metrics
```

## 目录概览

- [cmd/manager/main.go](cmd/manager/main.go): 启动 controller-manager、健康检查和 Webhook 接收器
- [api/v1alpha1/healingrequest_types.go](api/v1alpha1/healingrequest_types.go): `HealingRequest` CRD 定义、默认值和校验约束
- [internal/ingestion](internal/ingestion): Alertmanager Webhook 接入与幂等去重
- [internal/controllers](internal/controllers): `HealingRequest` Reconciler
- [internal/healing](internal/healing): 编排、快照与最小工作负载适配器
- [internal/safety](internal/safety): 维护窗口、速率限制、爆炸半径、熔断等门禁逻辑
- [internal/observability](internal/observability): 审计、事件和指标
- [internal/agent](internal/agent): Agent v1 分诊契约、输入分层与 Telegram 通知模板
- [charts/kube-sentinel](charts/kube-sentinel): Helm values 与 schema
- [docs/release-and-rollback.md](docs/release-and-rollback.md): 发布、灰度和回滚说明
- [docs/ops-console.md](docs/ops-console.md): Headlamp 对象视图与 Grafana 指标视图接入说明
- [docs/agent-triage.md](docs/agent-triage.md): Agent v1 五段式输出、输入分层与 Telegram 通知说明

## 环境要求

- Go 1.22+
- 一个可访问的 Kubernetes 集群
- 本地调试时建议直接使用 minikube
- `kubectl`
- `golangci-lint`
- `goimports`

## 快速开始

### 1. 运行质量检查

```bash
make test
make race
make vet
make lint
make quality-gate
```

### 2. 构建最小安装入口

仓库现在提供了最小镜像构建与测试环境安装入口：

```bash
bash ./scripts/install-minimal.sh
```

默认行为：

- 自动构建本地镜像 `kube-sentinel/controller:latest`
- 当前 context 是 minikube 时，自动执行 `minikube image load`
- 安装 `HealingRequest` CRD
- 部署 `kube-sentinel` ServiceAccount、RBAC、Deployment 与 Service
- 输出下一步连接 manager 与执行 smoke 的命令提示

常用覆盖项：

```bash
KUBE_SENTINEL_NAMESPACE=kube-sentinel-system \
KUBE_SENTINEL_IMAGE=example.local/kube-sentinel:test \
KUBE_SENTINEL_BUILD_IMAGE=false \
bash ./scripts/install-minimal.sh
```

如果你只想查看渲染后的清单，不实际写入集群：

```bash
KUBE_SENTINEL_INSTALL_DRY_RUN=true bash ./scripts/install-minimal.sh
```

如果你不需要验证本地未发布改动，而是希望直接部署官方预构建镜像，可以跳过本地 build：

```bash
KUBE_SENTINEL_IMAGE=ghcr.io/yangyus8/kube-sentinel:v0.1.0 \
KUBE_SENTINEL_BUILD_IMAGE=false \
bash ./scripts/install-minimal.sh
```

官方镜像使用边界：

- `ghcr.io/yangyus8/kube-sentinel:vX.Y.Z`：稳定版，适合共享测试环境或可重复部署场景。
- `ghcr.io/yangyus8/kube-sentinel:latest`：始终指向最近一次稳定版发布，不承载预发布结果。
- `ghcr.io/yangyus8/kube-sentinel:vX.Y.Z-rc.N` / `-beta.N`：预发布镜像，仅用于联调和发布前验证。
- `kube-sentinel/controller:latest`：本地构建默认镜像，仅用于当前工作区快速迭代，不应作为共享环境基线。

### 2.5 运行模式

默认安装会以最小自动动作模式启动：仅允许 Deployment L1 自动尝试。

- `KUBE_SENTINEL_RUNTIME_MODE=minimal`：启用极简 V1 runtime，默认值。
- `KUBE_SENTINEL_READ_ONLY_MODE=false`：允许 Deployment L1 自动动作，默认值。
- `KUBE_SENTINEL_READ_ONLY_MODE=true`：退回只读模式，仅生成 incident、审计和建议，不执行自动写动作。

只读模式示例：

```bash
KUBE_SENTINEL_READ_ONLY_MODE=true bash ./scripts/install-minimal.sh
```

### 2.6 首发发布执行路径

首个可交付版本遵循固定顺序：预生产验证 -> RC 发布 -> pilot/cutover -> 稳定版发布。

- 首发阻断范围只覆盖 Deployment 自动闭环；StatefulSet 自动写动作与额外 UI 查询层不属于首发放行前置条件。
- 统一预演入口：`bash ./scripts/v1-release-execution.sh`
- 首发检查表：见 [docs/v1-release-checklist.md](docs/v1-release-checklist.md)
- 发布与回滚说明：见 [docs/release-and-rollback.md](docs/release-and-rollback.md)

RC 预演示例：

```bash
V1_RELEASE_STAGE=rc \
V1_RELEASE_VERSION_TAG=v1.0.0-rc.1 \
bash ./scripts/v1-release-execution.sh
```

稳定版放行示例：

```bash
V1_RELEASE_STAGE=stable \
V1_RELEASE_VERSION_TAG=v1.0.0 \
V1_RELEASE_RC_TAG=v1.0.0-rc.1 \
V1_RELEASE_PREPROD_VERIFIED=true \
V1_RELEASE_PILOT_VERIFIED=true \
V1_RELEASE_GO_LIVE_VERIFIED=true \
bash ./scripts/v1-release-execution.sh
```

预演和放行记录默认归档到 `.tmp/v1-release-execution/<stage>-<version>/`，至少包含发布 trace、delivery pipeline 输出和 release plan 摘要。

### 2.7 Agent v1 分诊与 Telegram 通知

Agent v1 负责夜间值班分诊，而不是自治执行。默认输出固定为五段：

- `what happened`
- `what runtime did`
- `current focus`
- `next steps`
- `handoff`

同时，系统会把真实 runtime phase 翻译成更适合值班的 `oncall state`：

- `Pending` / `PendingVerify` -> `observing`
- `Blocked` / `L3` -> `blocked`
- `L1` / `Completed` -> `auto-tried`
- `Suppressed` -> `recovered`

V1 主动通知只支持 Telegram，并区分：

- `observing`
- `auto-tried`
- `blocked`
- `recovered`

默认会对 Telegram 通知做最小降噪：

- 同一 incident 在同一 `oncall state` 下不重复发送等价消息
- `observing` 采用低噪音策略
- Telegram 不可用时抑制重复失败事件

详细说明见 [docs/agent-triage.md](docs/agent-triage.md)。

### 3. 准备本地集群上下文

如果你使用 minikube，本地启动前先确认当前 kube context 已经指向它：

```bash
minikube start
kubectl config use-context minikube
kubectl config current-context
kubectl cluster-info
```

预期结果：

- `kubectl config current-context` 输出 `minikube`
- `kubectl cluster-info` 能返回 apiserver 地址

如果这里失败，`go run ./cmd/manager` 也会失败，因为 controller-runtime 在启动时就需要加载 Kubernetes client 配置。

如果你在 Linux 上使用 `minikube + podman/cri-o`，并且宿主机访问外网依赖代理，需要特别注意两点：

- 不要把节点内代理地址写成 `localhost`；对 minikube 节点而言，这会指向节点自身而不是宿主机。
- 需要把宿主机可达的代理地址传给 minikube 启动环境，否则系统组件可能卡在 `ImagePullBackOff`，节点会一直 `NotReady`。

示例：

```bash
export HTTP_PROXY=http://<host-reachable-proxy>:7890
export HTTPS_PROXY=http://<host-reachable-proxy>:7890
export NO_PROXY=127.0.0.1,localhost,10.96.0.0/12,10.244.0.0/16,.svc,.cluster.local

minikube start --driver=podman \
  --container-runtime=cri-o \
  --embed-certs \
  --docker-env HTTP_PROXY="$HTTP_PROXY" \
  --docker-env HTTPS_PROXY="$HTTPS_PROXY" \
  --docker-env NO_PROXY="$NO_PROXY"
```

启动后先确认：

- `kubectl get nodes` 中节点状态为 `Ready`
- `kubectl -n kube-system get pods` 中 `coredns`、网络插件、`storage-provisioner` 均为 `Running`

如果这些基础组件还没起来，先不要继续验证 Kube-Sentinel，本地闭环一定会出现误报。

### 4. 本地开发回路

统一本地入口：

```bash
bash ./scripts/dev-local-loop.sh check
```

这个入口会统一完成：

- kube context 与 apiserver 连通性检查
- `kube-system` 基础组件就绪检查
- `HealingRequest` CRD 缺失时自动补装
- `default/demo-app` demo workload 缺失时自动补齐
- 输出后续运行本地 manager 或连接集群 manager 的下一步命令

本地直接运行 manager：

```bash
bash ./scripts/dev-local-loop.sh run-local
```

如果 `8080`、`8081` 或 `8090` 端口被占用，这个入口会直接阻断并提示修复。

如果控制器已经以 Pod 形式运行在集群里，可以复用同一个入口连接集群中的 Service：

```bash
bash ./scripts/dev-local-loop.sh connect-cluster
```

对象视图验证和指标视图验证请分开执行：

- 对象视图验证：使用 Headlamp 或 kubectl 查看 `HealingRequest` 列表与详情，确认 `Phase`、`Action`、`Reason`、`Recommendation`、`Correlation` 等字段可直接识别。
- 指标视图验证：确认 `kube-sentinel-metrics` Service 已暴露，再让 Prometheus 抓取并把 [docs/ops-console.md](docs/ops-console.md) 中的 Grafana dashboard 导入进来。

完整控制台接入说明见 [docs/ops-console.md](docs/ops-console.md)。

### 5. 安装 CRD

在启动 manager 或发送 webhook 之前，先安装 HealingRequest CRD：

```bash
kubectl apply -f config/crd/_healingrequests.yaml
kubectl get crd healingrequests.kubesentinel.io
```

如果缺少这一步，Webhook 虽然可能接收到请求，但 `HealingRequest` 无法落库，闭环验证会直接失败。

如果你已经执行了 [scripts/install-minimal.sh](scripts/install-minimal.sh)，这一节通常已经由统一入口完成。

### 6. 启动控制器

Kube-Sentinel 当前通过 controller-runtime manager 运行，并在本地暴露以下端口：

- metrics: `:8080`
- healthz/readyz: `:8081`
- webhook: `:8090`

最小安装清单还会额外创建 `kube-sentinel-metrics` Service，供 Prometheus 使用标准抓取方式接入 metrics，而不需要把 Grafana 联调建立在临时端口转发之上。

本地启动：

```bash
go run ./cmd/manager
```

现在启动阶段会打印结构化日志。成功启动时，至少应该能看到类似信息：

```text
starting kube-sentinel manager
loaded kubernetes client configuration
registered HealingRequest controller
starting Alertmanager webhook receiver
registered health and readiness checks
```

如果启动失败，终端会直接打印具体失败原因，常见情况包括：

- kubeconfig 未配置
- 当前 context 不存在
- apiserver 不可达
- 本地端口 `8080`、`8081` 或 `8090` 已被占用

如果 manager 已经启动成功，但随后看到类似 `workload default/demo-app not found as Deployment or StatefulSet` 的日志，这表示控制器已经开始处理某个 `HealingRequest`，只是告警指向的目标工作负载在集群中不存在。

### 7. 发送一个测试告警

Webhook 路径：

```text
POST /alertmanager/webhook
```

最小示例：

```json
{
  "alerts": [
    {
      "status": "firing",
      "fingerprint": "demo-fp-1",
      "labels": {
        "workload_kind": "Deployment",
        "namespace": "default",
        "name": "demo-app",
        "alertname": "CrashLoopBackOff",
        "severity": "Critical"
      },
      "annotations": {
        "summary": "demo deployment is unhealthy"
      }
    }
  ]
}
```

发送示例：

```bash
curl -X POST http://127.0.0.1:8090/alertmanager/webhook \
  -H 'Content-Type: application/json' \
  -d '{
    "alerts": [
      {
        "status": "firing",
        "fingerprint": "demo-fp-1",
        "labels": {
          "workload_kind": "Deployment",
          "namespace": "default",
          "name": "demo-app",
          "alertname": "CrashLoopBackOff",
          "severity": "Critical"
        },
        "annotations": {
          "summary": "demo deployment is unhealthy"
        }
      }
    ]
  }'
```

### 8. 查看生成的 HealingRequest

Receiver 会将告警映射成 `HealingRequest`。对象名默认形如：

```text
hr-<workload-name>
```

例如：

```bash
kubectl get healingrequests -n default
kubectl get healingrequest hr-demo-app -n default -o yaml
```

重点关注这些状态字段：

- `status.phase`
- `status.lastAction`
- `status.lastGateDecision`
- `status.blockReasonCode`
- `status.lastError`
- `status.nextRecommendation`
- `status.correlationKey`

如果是在单节点、低副本的本地集群上做第一次闭环联调，还要额外看：

- `status.blockReasonCode`
- `status.namespaceBlockRate`
- `status.lastGateDecision`

默认 `blastRadius.maxPodPercentage=10` 对本地小集群通常过严。比如默认 namespace 只有 4 个 Pod，而目标 Deployment 有 3 个副本时，影响比例是 $75\%$，一定会被门禁阻断。这不是控制器异常，而是安全策略按设计生效。

统一本地 smoke 入口：

```bash
bash ./scripts/drill-runtime-closed-loop.sh default
```

这个脚本会固定执行两条路径：

- 默认保守配置下，验证 `HealingRequest` 创建与 `gate_blocked` / `blast radius exceeded` 阻断语义
- 只对当前 `HealingRequest` 临时放宽 `spec.blastRadius.maxPodPercentage` 与 soak 配置，验证 `PendingVerify -> Completed` 成功闭环

默认情况下，脚本会为每次运行创建一个临时 smoke Deployment，并在结束时自动清理，避免重复执行时命中上一轮对象的幂等窗口。若你想保留现场供排查，可设置 `KUBE_SENTINEL_KEEP_SMOKE_RESOURCES=true`。

如果 controller 是以 Pod 形式运行在集群里，可以先开一个终端执行：

```bash
bash ./scripts/dev-local-loop.sh connect-cluster
```

然后在另一个终端执行 smoke。

本地 smoke 约定：

- 使用更多基础 Pod 拉高总量，或者
- 仅临时把当前验证对象的 `spec.blastRadius.maxPodPercentage` 提高到适合本地环境的值，例如 `100`

联调完成后应恢复更保守的阈值。不要把本地 smoke 中的放宽值抄回 [charts/kube-sentinel/values.yaml](charts/kube-sentinel/values.yaml) 或正式环境默认配置。

## 首发行为说明

### Deployment

当前唯一自动写动作是 Deployment L1：

- 动作类型：`rollout restart`
- 成功前置：必须通过门禁并成功创建快照
- 失败语义：一旦 L1 失败，立即阻断并转人工介入建议，不继续自动升级到 L2/L3

### StatefulSet

当前首发版本中：

- 允许接入
- 允许只读评估
- 默认不允许自动写动作

## 默认配置

Helm 默认值位于 [charts/kube-sentinel/values.yaml](charts/kube-sentinel/values.yaml)。首发默认值包括：

- `workloadKind: Deployment`
- `idempotencyWindowMinutes: 5`
- `rateLimit.maxActions: 3`
- `rateLimit.windowMinutes: 10`
- `blastRadius.maxPodPercentage: 10`
- `circuitBreaker.objectFailureThreshold: 3`
- `circuitBreaker.domainFailureThreshold: 10`
- `circuitBreaker.cooldownMinutes: 10`
- `snapshotPolicy.enabled: true`

manager 运行时也支持通过环境变量加载最小监听配置：

- `KUBE_SENTINEL_METRICS_BIND_ADDRESS`
- `KUBE_SENTINEL_HEALTH_PROBE_BIND_ADDRESS`
- `KUBE_SENTINEL_WEBHOOK_BIND_ADDRESS`
- `KUBE_SENTINEL_RUNTIME_MODE`
- `KUBE_SENTINEL_READ_ONLY_MODE`
- `KUBE_SENTINEL_TELEGRAM_BOT_TOKEN`
- `KUBE_SENTINEL_TELEGRAM_CHAT_ID`
- `KUBE_SENTINEL_TELEGRAM_BASE_URL`（可选，默认 `https://api.telegram.org`）

启用 Telegram 主动通知示例：

```bash
KUBE_SENTINEL_TELEGRAM_BOT_TOKEN=<token> \
KUBE_SENTINEL_TELEGRAM_CHAT_ID=<chat-id> \
bash ./scripts/install-minimal.sh
```
- `KUBE_SENTINEL_WEBHOOK_PATH`

[config/install/kube-sentinel.yaml](config/install/kube-sentinel.yaml) 使用这组环境变量把安装清单中的默认监听地址显式传给控制器。

Schema 约束位于 [charts/kube-sentinel/values.schema.json](charts/kube-sentinel/values.schema.json)。

## 可观测性

### Metrics

metrics 由 controller-runtime 暴露在 `:8080`。

当前首发版本重点关注：

- `kube_sentinel_triggers_total`
- `kube_sentinel_success_total`
- `kube_sentinel_healing_failures_total`
- `kube_sentinel_circuit_breaks_total`
- `kube_sentinel_snapshot_creates_total`
- `kube_sentinel_deployment_l1_results_total`
- `kube_sentinel_readonly_blocks_total`
- `kube_sentinel_strategy_duration_seconds`

### 事件与审计

- 运行时事件通过 K8s Event 和内存事件 sink 记录
- 审计记录包含触发源、目标对象、阶段、动作、结果、门禁判定和建议动作
- `correlationKey` 用于串联告警、状态、事件和审计证据

## 质量门禁

推荐统一通过以下命令验证可交付性：

```bash
make quality-gate
```

该命令会串联：

- 单元测试
- 核心路径 race 检查
- `go vet`
- `golangci-lint`
- CRD 一致性检查
- API/Helm 同步检查

## 灰度与回滚

首发版本不是“默认全量自动化”，而是“默认保守放量”。

推荐顺序：

1. 先在低风险命名空间灰度启用
2. 验证 webhook 接入、门禁阻断和事件链路
3. 观察失败率、熔断次数、快照失败率
4. 确认 `StatefulSet` 仍保持只读
5. 稳定后再扩大 Deployment 覆盖范围

回滚原则：

1. 先关闭自动写路径
2. 保留只读评估与审计
3. 根据最近快照和审计记录执行人工恢复

更多细节见 [docs/release-and-rollback.md](docs/release-and-rollback.md)。

## 相关命令

```bash
make test
make race
make vet
make fmt
make lint
make quality-gate
make delivery-pipeline
make delivery-trend-report
```

## 启动排错

如果本地仍然无法启动，优先按下面顺序检查：

```bash
kubectl config current-context
kubectl cluster-info
kubectl get ns
```

如果是 minikube，还可以继续检查：

```bash
minikube status
```

如果端口冲突，可以先看本地占用：

```bash
ss -ltnp | grep -E ':8080|:8081|:8090'
```

## 当前局限

- 当前提供的是测试环境最小安装入口，不是完整生产安装基线
- 当前 README 以首发闭环为中心，不覆盖后续所有 roadmap 能力
- 当前首发版本的重点是“Deployment 安全 L1”，不是最终多级自愈终态

## 参考文档

- [docs/release-and-rollback.md](docs/release-and-rollback.md)
- [openspec/specs/deployment-safe-l1-mvp/spec.md](openspec/specs/deployment-safe-l1-mvp/spec.md)
- [openspec/specs/deployment-healing-orchestration/spec.md](openspec/specs/deployment-healing-orchestration/spec.md)
- [openspec/specs/api-contract-governance/spec.md](openspec/specs/api-contract-governance/spec.md)
