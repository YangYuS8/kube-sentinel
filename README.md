# Kube-Sentinel

Kube-Sentinel 是一个基于事件驱动的 Kubernetes 故障自愈控制器。

当前首发版本聚焦在一个明确收敛的能力边界上：

- 通过 Alertmanager Webhook 接收故障事件
- 将事件映射为 `HealingRequest`
- 对 Deployment 执行安全优先的 L1 最小影响动作
- 在任何自动写操作前执行门禁判定与快照校验
- 输出结构化审计记录、运行时事件和指标

它不是完整可观测平台，也不是全自动发布系统。当前版本的目标，是交付第一个可持续发布、可验证、可回滚的自动处置闭环。

## 当前支持范围

首发版本已经完成的能力：

- `Deployment` 自动处置闭环
- Alertmanager Webhook 接入与事件幂等去重
- `HealingRequest` 默认值、校验约束和状态语义
- L1 写动作前的安全门禁：维护窗口、速率限制、爆炸半径、熔断
- 写前快照创建失败即阻断
- K8s Event、结构化审计和 Prometheus 指标输出
- 质量门禁：`go test`、`race`、`vet`、`golangci-lint`、CRD/Helm 一致性检查

当前明确不属于首发范围的能力：

- `Deployment` 的 L2/L3 自动升级
- `StatefulSet` 自动写动作
- 复杂发布门禁自动化
- 快照恢复编排的完整生产流程

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
        +--> Audit / Event / Metrics
```

## 目录概览

- [cmd/manager/main.go](cmd/manager/main.go): 启动 controller-manager、健康检查和 Webhook 接收器
- [api/v1alpha1/healingrequest_types.go](api/v1alpha1/healingrequest_types.go): `HealingRequest` CRD 定义、默认值和校验约束
- [internal/ingestion](internal/ingestion): Alertmanager Webhook 接入与幂等去重
- [internal/controllers](internal/controllers): `HealingRequest` Reconciler
- [internal/healing](internal/healing): 编排、快照、回滚与工作负载适配器
- [internal/safety](internal/safety): 维护窗口、速率限制、爆炸半径、熔断等门禁逻辑
- [internal/observability](internal/observability): 审计、事件和指标
- [charts/kube-sentinel](charts/kube-sentinel): Helm values 与 schema
- [docs/release-and-rollback.md](docs/release-and-rollback.md): 发布、灰度和回滚说明

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
