## 上下文

当前 Deployment 路径在 L1 rollout restart 失败或快照创建失败后，会直接把 `deploymentL2Decision` 写成 `not-allowed-in-mvp`，并把 `deploymentL2Result` 标记为 `skipped`。这与主规格中已经存在的 Deployment L2 健康版本回滚要求不一致，也让最核心的通用工作负载仍停留在“失败后仅人工接管”的半成品状态。

与此同时，仓库已经具备这次实现所需的多数基础部件：

- `DeploymentPolicy`、`HealthyRevision`、快照策略、熔断与爆炸半径约束已经在 CRD 中定义。
- `SelectLatestHealthyRevision` 与 `EvaluateL2RollbackGate` 已存在，可直接作为候选选择和风险门禁的基础。
- `Adapter.ListRevisions`、`Adapter.ValidateRevisionDependencies`、`Adapter.RollbackToRevision` 已被 StatefulSet L2 复用，Deployment 可以沿用同一抽象。
- `Metrics` 中已经暴露 Deployment L2 success/fallback/degraded 计数器，但主链路还没有真正驱动这些指标。

这说明本次变更不应再扩展新的大块架构，而应把 Deployment L2 从占位状态收敛为真实可执行的领域流程，并与已有的快照、审计和门禁机制对齐。

## 目标 / 非目标

**目标：**

- 让 Deployment 在 L1 失败后进入真实的 L2 健康版本回滚流程，而不是直接 `skipped`。
- 复用现有修订历史、候选健康判定、依赖校验与快照恢复能力，保持实现最小增量。
- 明确 L2 成功、候选不足、门禁阻断、回滚失败、恢复失败这几类结果在 status、事件、审计和指标上的输出约定。
- 为后续灰度启用提供可验证的失败路径测试，覆盖幂等、回滚性、门禁和恢复语义。

**非目标：**

- 不在这次变更中扩展新的外部 API 版本或新增全新的 CRD 字段。
- 不在这次变更中重做 Deployment L1 算法、一般性门禁算法或 Alertmanager 接入。
- 不把 StatefulSet L2 与 Deployment L2 做统一重构，只抽取确实可共享的最小逻辑。
- 不在这次变更中实现更激进的 L3 自动化，仅输出结构化人工介入建议。

## 决策

### 决策 1：新增 `processDeploymentL2`，而不是继续把 L2 逻辑内嵌在 L1 分支里

- 选择：参照 `processStatefulSetL2`，为 Deployment 提供独立的 L2 处理函数，并在 L1 失败时传入预先创建的 snapshot 与 runtimeInput。
- 原因：Deployment L2 的失败分支已经包含候选判定、风险门禁、依赖校验、执行回滚、恢复快照、冻结/降级建议等多个步骤，继续内嵌会让主 `Reconcile` 分支难以维护和测试。
- 备选方案：
  - 继续在现有 Deployment L1 失败分支里逐步插入逻辑：改动看似少，但会让阶段迁移、审计和指标更新分散在多个分支。
  - 直接抽象通用 `processWorkloadL2`：当前 Deployment 与 StatefulSet 的冻结、证据和降级语义仍有差异，过早统一会增加风险。

### 决策 2：Deployment L2 候选继续基于现有修订历史，选择最近的健康版本

- 选择：调用 `Adapter.ListRevisions` 获取修订记录，使用 `SelectLatestHealthyRevision` 按时间倒序选择最新健康 revision，并在执行前调用 `ValidateRevisionDependencies`。
- 原因：这条链路已经被现有领域模型和适配器支持，且与 `healthyRevision` 策略保持一致；不需要引入新的候选来源或额外的持久化层。
- 备选方案：
  - 从 `status.lastHealthyRevision` 直接回滚：信息不足时容易陈旧，而且不能替代实时依赖校验。
  - 引入 Deployment 专属健康候选缓存：复杂度更高，但当前没有证据说明现有修订记录不足以支撑 MVP 的 L2。

### 决策 3：L2 风险门禁沿用 `DeploymentPolicy` 和现有回滚窗口评估，不新增硬编码阈值

- 选择：在进入 L2 前，根据历史窗口和 `deploymentPolicy.l2MaxDegradeRatePercent` 调用 `EvaluateL2RollbackGate`。若门禁阻断，则降级到 L3 并输出结构化建议。
- 原因：`openspec/config.yaml` 明确禁止硬编码业务阈值。现有 `DeploymentPolicy` 已经提供阈值承载位，本次只需把它真正接入运行链路。
- 备选方案：
  - 直接允许所有 L1 失败进入 L2：会削弱安全优先原则。
  - 先写死默认阈值，后续再暴露到 CRD：违反配置化约束，也会带来后续兼容负担。

### 决策 4：L2 回滚失败后必须尝试恢复 L1 前快照，并把恢复结果折叠到同一次处置证据中

- 选择：若 `RollbackToRevision` 失败，则立刻调用 `restoreSnapshot` 恢复 L1 前快照，使用现有恢复影响分析生成 `blockReasonCode`、`lastGateDecision` 和 `nextRecommendation`，最终进入 Blocked 或 L3 人工介入语义。
- 原因：配置要求任何动作必须可回滚、可撤销。Deployment L2 如果不能在失败时恢复 pre-L1 状态，就无法满足该要求。
- 备选方案：
  - 回滚失败后直接进入 L3，不尝试恢复：会留下不确定的中间态。
  - 为 Deployment 单独实现另一套恢复逻辑：与现有 snapshot/restore 语义分叉，没有必要。

### 决策 5：优先复用现有状态字段，不引入新的 CRD 字段

- 选择：使用已有的 `deploymentL2Candidate`、`deploymentL2Decision`、`deploymentL2Result`、`lastHealthyRevision`、`snapshotRestoreResult`、`nextRecommendation`、`blockReasonCode` 和 `lastGateDecision` 承载新行为。
- 原因：这些字段已经能表达候选、结果、恢复和建议，且 `status.nextRecommendation` 已被 API 合约强制要求。新增字段会扩大 API 变更面与一致性检查成本。
- 备选方案：
  - 新增 Deployment 专属失败细分字段：表达更细，但会放大 CRD/schema/Helm 约束同步成本。

## 风险 / 权衡

- [风险] Deployment 与 StatefulSet L2 虽然相似，但结果语义不完全一致，照搬可能引入不符合 Deployment 现状的状态文案。
  → 缓解：只复用候选选择、依赖校验和 snapshot restore 模式，Deployment 自己定义 decision/result/recommendation 字面值。
- [风险] 历史修订不足时，L2 会频繁降级到 L3，可能让用户感觉“功能没有生效”。
  → 缓解：在状态和事件中明确给出 no-healthy-candidate、dependency-validation-failed 等原因，避免与实现缺失混淆。
- [风险] 引入 L2 后，既有测试对 `not-allowed-in-mvp` 的断言会大面积失效。
  → 缓解：先按结果类别重写测试基线，再增加失败路径覆盖，而不是用字符串替换式修修补补。
- [风险] 回滚失败后的 snapshot restore 仍可能失败，导致对象停在更危险的中间态。
  → 缓解：统一使用恢复影响分析产出 recommendation，并在审计和指标里区分 rollback failed 与 restore failed。

## Migration Plan

1. 先在领域层引入 Deployment L2 处理函数和状态迁移，保留外部 API 版本不变。
2. 调整现有 Orchestrator 测试，把 Deployment L1 失败后的预期从 `skipped/not-allowed-in-mvp` 迁移到真实 L2 结果集合。
3. 补充 Deployment L2 成功、候选缺失、门禁阻断、回滚失败且恢复成功、回滚失败且恢复失败等测试。
4. 更新发布与回滚文档，明确该能力仍应通过灰度启用和本地 smoke/集群测试验证后再放量。
5. 若实现存在回归，可临时回退到旧的 L1-only 行为，但必须同时撤回规格增量，避免规格与实现再次分叉。

## Open Questions

- Deployment L2 的窗口统计应直接复用现有 `DeploymentPolicy` 成功率/降级率字段，还是需要在实现时再明确一套最小取样规则？
- Deployment 在 L2 成功后是否需要单独进入 `PendingVerify`，还是保持与现有 L1/StatefulSet L2 一致，动作成功后直接 `Completed` 并依赖上层告警再次驱动？
- 当前是否需要为 Deployment L2 引入显式 enable 开关，还是保持“只要存在健康候选且门禁允许就自动尝试”的默认语义？
