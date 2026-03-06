## 为什么

当前 Kube-Sentinel 已具备从告警接入到自愈编排的主链路骨架，但能力范围仍然偏大，包含 Deployment 与 StatefulSet、多级处置、发布门禁、快照恢复与多类可观测性要求。继续并行推进整套能力会放大范围漂移与验收不清的问题，难以形成第一个可持续交付、可演示、可回滚的成品。

因此需要先冻结一个最小但真实的纵向切片：以 Deployment 为唯一可写对象，围绕 L1 最小影响动作形成从告警接入、门禁判定、快照前置校验、动作执行到审计证据的完整闭环。这样可以尽快建立首个可交付基线，并为后续 L2/L3、StatefulSet 和更复杂治理能力提供稳定落脚点。

## 变更内容

- 将首个交付范围限定为 Deployment 的安全 L1 自动处置闭环，不在本变更中扩展 StatefulSet 可写能力。
- 明确 L1 的成功路径：接收 Alertmanager Webhook、创建或更新 HealingRequest、通过安全门禁、创建写前快照、执行单一 Deployment L1 动作、输出审计与事件证据。
- 明确 L1 的失败路径：任一前置校验失败时必须阻断写操作，保持只读评估或人工介入建议，不升级到更激进的自动动作。
- 冻结第一成品的非目标范围：Deployment L2/L3 自动升级、StatefulSet 条件可写、复杂发布门禁自动化、快照恢复编排、跨能力交付运营报表。
- 将验收重点前移到可持续交付：要求围绕幂等、维护窗口、速率限制、爆炸半径、熔断、快照失败阻断和状态语义建立可重复验证的测试与交付门槛。

## 功能 (Capabilities)

### 新增功能

- `deployment-safe-l1-mvp`: 定义首个 Deployment L1 安全闭环成品的范围、验收基线与非目标边界。

### 修改功能

- `alertmanager-webhook-ingestion`: 收敛首个成品对 Deployment Webhook 接入、幂等去重和上下文映射的最小要求。
- `deployment-healing-orchestration`: 收敛第一成品只允许 Deployment L1 自动动作，失败时阻断升级并输出可解释证据。
- `durable-snapshot-and-restore`: 强化“任何 L1 写动作前必须成功创建持久快照，否则禁止写操作”的首发要求。
- `tiered-circuit-breaking`: 明确 Deployment L1 首发版本必须纳入维护窗口、速率限制、爆炸半径和熔断联动的只读阻断逻辑。
- `api-contract-governance`: 补强第一成品所需的状态语义最小集合，确保阶段、最近动作、失败原因和下一步建议稳定可检索。

## 影响

- 受影响代码主要位于 [cmd/manager/main.go](cmd/manager/main.go)、[internal/ingestion/receiver.go](internal/ingestion/receiver.go)、[internal/controllers/healingrequest_controller.go](internal/controllers/healingrequest_controller.go)、[internal/healing/orchestrator.go](internal/healing/orchestrator.go)、[internal/safety](internal/safety) 和 [internal/observability](internal/observability)。
- 受影响 API 为 [api/v1alpha1/healingrequest_types.go](api/v1alpha1/healingrequest_types.go) 中与 L1 门禁、快照和状态输出相关的字段契约。
- 受影响交付门槛包括测试、静态检查、CRD 一致性与 Helm 约束同步。
- 本变更不引入新的外部依赖；重点在于收敛现有能力、明确首发边界，并为后续增量切片保留兼容演进路径。
