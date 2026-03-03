## 为什么

当前 Kube-Sentinel 已具备主线A闭环能力，但仍存在关键“演示态”实现：业务阈值与时间窗口硬编码、门禁输入非真实运行态、Deployment 适配器为占位实现、观测输出以内存对象为主。若直接进入更复杂能力扩展，会放大生产风险并违反现有配置与安全约束。

现在需要先完成 P0 生产化收敛，使现有闭环在真实运行场景下“可配置、可验证、可观测、可回滚”。

## 变更内容

- 移除编排与门禁路径中的业务硬编码，将幂等窗口、速率限制、爆炸半径、熔断阈值统一由 CRD/配置驱动。
- 将门禁判定输入接入真实运行态数据（动作计数、受影响 Pod 数、集群 Pod 基数），替换固定示例值。
- 落地 DeploymentAdapter 的生产实现：Revision 检索与回滚执行（含失败恢复语义）。
- 将观测能力从内存计数/内存事件扩展为可运维采集输出（Prometheus 指标与 K8s Event 对齐）。
- 完善失败路径与边界测试，确保质量门禁下可稳定演进。

## 功能 (Capabilities)

### 新增功能

- `runtime-production-hardening`: 约束闭环从 MVP 进入生产可用基线（配置化、真实门禁输入、适配器落地、观测输出一致性）。

### 修改功能

- `deployment-healing-orchestration`: 编排链路改为完全配置驱动，门禁输入来源改为运行态事实数据。
- `healthy-revision-rollback`: 将最近健康 Revision 选择与回滚动作绑定到真实 Deployment Revision 数据源，并明确失败恢复证据。
- `tiered-circuit-breaking`: 熔断阈值与恢复信息输出改为配置/状态一致，消除固定阈值初始化。
- `alertmanager-webhook-ingestion`: 幂等窗口与请求映射策略改为参数化并与请求配置对齐。
- `runtime-closed-loop-validation`: 验收基线增加“无硬编码阈值、真实门禁输入、可采集观测输出”的强断言。

## 影响

- 受影响代码：`internal/ingestion`、`internal/healing`、`internal/safety`、`internal/controllers`、`internal/observability`。
- 受影响 API/配置：`api/v1alpha1` 字段语义与默认值使用路径、Helm values 与 schema 约束。
- 受影响运行系统：Prometheus 指标采集、K8s Event 检索、Alertmanager 接入一致性。
- 交付影响：需强化失败路径单测与并发路径检查，确保 `go test ./...`、`go test -race`、`go vet`、`golangci-lint`、CRD 一致性检查稳定通过。
