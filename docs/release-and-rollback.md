# 发布与回滚说明

## deployment-safe-l1-mvp 首发说明

- 首发只开放 Deployment 的 L1 最小影响动作，默认动作是 `rollout restart`。
- StatefulSet 在首发阶段仍按只读评估处理，不纳入自动写路径验收。
- Deployment 的 L2/L3 自动升级与复杂发布门禁不属于首发范围；L1 失败时应输出阻断证据并转人工介入。
- 测试环境统一入口使用 [scripts/install-minimal.sh](scripts/install-minimal.sh) 与 [scripts/dev-local-loop.sh](scripts/dev-local-loop.sh)；它们面向联调与 minikube，不直接等同生产安装基线。
- 首发灰度必须优先验证：Webhook 接入幂等、维护窗口阻断、速率限制、爆炸半径、熔断、写前快照失败阻断。
- 首发回滚优先级：先关闭自动写路径，再保留只读评估与审计链路，最后根据最近快照与审计记录执行人工恢复。

## 灰度启用策略

0. 合并前必须执行统一交付门禁：`make quality-gate`。
1. 首先在低风险命名空间启用 webhook 接入与对象级熔断。
2. 校验运行参数为声明式配置驱动（幂等窗口、限频、爆炸半径、熔断阈值）。
3. 本地联调优先执行 `bash ./scripts/drill-runtime-closed-loop.sh default`，确认默认阻断路径与单次放宽后的成功路径都通过。
4. 观察关键指标（失败率、回滚次数、熔断次数）24 小时。
5. 通过配置开关启用域级熔断。
6. 校验 K8s Event 与审计记录可通过 correlation key 串联。
7. 启用保守模式后，校验 `PendingVerify` / `Suppressed` 状态、影子执行说明与命名空间预算阻断证据。
8. 启用 StatefulSet 接入时，先确认默认仅 `read-only`，阻断原因包含 `statefulset_readonly`。
9. 启用 StatefulSet Phase 2 时，必须同时配置：`controlledActionsEnabled=true`、`allowedNamespaces`、`approvalAnnotation`、`freezeWindowMinutes`。
10. 灰度期间必须观测以下阈值：误动作率 < 1%、回退率 < 5%、冻结触发率 < 5%。任一越线立即回退只读。
11. 启用 StatefulSet Phase 3（L2）时，必须同时开启 `statefulSetPolicy.l2RollbackEnabled=true`，并校验 L2 候选窗口与降级阈值参数。
12. Phase 3 灰度期间重点观测：L2 成功率、L2 失败回退率、L2 降级率；任一连续窗口越线应关闭 L2。
13. 启用持久快照时，必须配置 `snapshotPolicy.retentionMinutes`、`snapshotPolicy.restoreTimeoutSeconds`、`snapshotPolicy.maxSnapshotsPerWorkload` 并先在白名单命名空间灰度。
14. 快照灰度期间重点观测：`kube_sentinel_snapshot_creates_total{result="failure"}`、`kube_sentinel_snapshot_restores_total{result="failure"}`、`kube_sentinel_snapshot_restore_duration_seconds`。
15. SLO 灰度放量必须分层推进：仅当上一层 `stableWindowPassed=true` 且未命中 `rollbackConditionActive` 才允许进入下一层。
16. 阈值调优必须具备审批人记录（例如 oncall 值班人）；无审批记录视为无效变更。
17. 同一对象在观察窗口（`sampleWindowMinutes`）内禁止重复调优；需等待窗口结束后再提交。
18. 阈值调优前必须保留上一个有效阈值快照，越线回退时恢复该快照。
19. API 变更发布前必须声明兼容性分类：`backward-compatible` / `migration-required` / `version-bump-required`，且必须与门禁证据字段一致。
20. 当兼容性分类为 `migration-required` 时，必须提供迁移路径（如 runbook/ref）；缺失时禁止放量。
21. 当兼容性分类为 `version-bump-required` 时，必须提供版本切换窗口并完成值班审批；未审批时禁止放量。
22. 当 API 契约风险等级为 `high` 时，必须在发布窗口中显式审批（`QUALITY_GATE_RELEASE_WINDOW_APPROVED=true`）后方可推进。

## 质量门禁失败分类（示例）

- `QUALITY_GATE_RESULT=block`
- `QUALITY_GATE_CATEGORY=crd_consistency`
- `QUALITY_GATE_REASON=crd_generation_drift`
- `QUALITY_GATE_FIX_HINT=run: controller-gen ... && cp -r .tmp/crd/* config/crd/`

当输出为 `QUALITY_GATE_RESULT=allow` 时，表示可进入下一发布检查环节。

## API 兼容性迁移模板（最小）

- 兼容性分类：`backward-compatible` / `migration-required` / `version-bump-required`
- 受影响字段：`QUALITY_GATE_API_AFFECTED_FIELDS`
- 迁移方案引用：`QUALITY_GATE_API_MIGRATION_PLAN`
- 版本切换窗口：`QUALITY_GATE_VERSION_BUMP_WINDOW`
- 风险等级：`QUALITY_GATE_API_RISK_LEVEL`
- 发布判定：`QUALITY_GATE_RELEASE_DECISION`

## 发布就绪摘要读取（最小）

- 摘要字段：`QUALITY_GATE_RELEASE_READINESS_ACTION_TYPE`、`QUALITY_GATE_RELEASE_READINESS_RISK_LEVEL`、`QUALITY_GATE_RELEASE_READINESS_STRATEGY_MODE`、`QUALITY_GATE_RELEASE_READINESS_CIRCUIT_TIER`。
- 关键证据：`QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE`、`QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS`、`QUALITY_GATE_RELEASE_READINESS_RECENT_DRILL_SCORE`。
- 演练聚合：`QUALITY_GATE_DRILL_SUCCESS_RATE`、`QUALITY_GATE_DRILL_ROLLBACK_P95_MS`、`QUALITY_GATE_DRILL_GATE_BYPASS_COUNT`。
- 判定一致性：`QUALITY_GATE_RELEASE_READINESS_DECISION` 必须与 `QUALITY_GATE_RELEASE_DECISION` 一致。

## 持续交付三段式流水线（最小）

- 执行顺序固定：`quality gate -> preprod dry-run -> evidence archive`。
- 任一阶段失败必须立即终止，输出 `DELIVERY_PIPELINE_FAILED_STAGE` 与 `DELIVERY_PIPELINE_REASON`。
- 机器可读证据：`delivery-evidence.json`，至少包含 `result`、`category`、`reasonCode`、`fixHint`。
- 人可读摘要：`delivery-summary.txt`，用于值班快速判断与回退触发。
- 建议统一入口：`make delivery-pipeline`；必要时通过 `DELIVERY_PIPELINE_*` 环境变量注入 dry-run 与归档路径。

## V1 go-live 决策流程（最小）

- 五类闸门固定顺序：`quality -> stability -> drillRollback -> approvalFreeze -> auditIntegrity`。
- 优先级固定：`quality > stability > drillRollback > approvalFreeze > auditIntegrity`；多闸门同时失败时按优先级输出 `failureCategory`。
- 判定语义固定：任一闸门 `fail` 则 `DELIVERY_PIPELINE_DECISION=block`，全部 `pass` 才 `allow`。
- 预生产前置约束：`DELIVERY_PIPELINE_PREPROD_STATUS` 必须为 `allow` 且证据未过期，否则阻断。
- 回滚演练约束：`QUALITY_GATE_DRILL_SUCCESS_RATE` 不低于 `DELIVERY_PIPELINE_DRILL_MIN_SUCCESS_RATE`，且 `QUALITY_GATE_DRILL_ROLLBACK_P95_MS` 不高于 `DELIVERY_PIPELINE_DRILL_MAX_ROLLBACK_P95_MS`。
- 冻结窗口约束：窗口命中时（`start <= now <= end`）禁止人工覆盖放量。

## V1 pilot/cutover 流程（最小）

- 状态机固定：`pilot_prepare -> pilot_observe -> cutover_ready -> cutover_done`，失败或阻断进入 `cutover_blocked`。
- 合法迁移：禁止跨阶段直切（例如 `pilot_prepare -> cutover_done`），非法迁移必须输出 `invalid_stage_transition`。
- 批次前置检查：每个 pilot 批次放量前必须同时满足质量通过、证据完整、阶段合法。
- 观察窗口门禁：进入 `cutover_ready/cutover_done` 前必须 `DELIVERY_PIPELINE_OBSERVE_WINDOW_COMPLETED=true` 且观察时长达标。
- 自动回退触发：熔断触发、SLO 进入 `rollback_required`、证据不完整时必须输出 `cutover_auto_rollback`。

## cutover decision pack 契约（最小）

- 最小字段：`decision`、`failureCategory`、`pilotBatch`、`rollbackTarget`、`traceKey`、`approval.requiredLevel|providedLevel`、`timestamp`。
- 建议补充字段：`pilotStateCurrent`、`pilotStateTarget`、`rollbackEvidence`、`sloMatrixAction`、`handoff.handoffOwner`。
- 任一最小字段缺失或语义冲突时必须阻断放量。

## 值班交接契约（最小）

- 必填字段：`handoffOwner`、`approvalLevel`、`traceKey`、`rollbackCommandRef`、`handoffTimestamp`。
- 交接缺失时必须阻断切流，禁止“先放量后补录”。
- 冻结窗口内禁止人工覆盖放量，仅允许只读评估与告警。

## pilot 期间 SLO 触发矩阵（最小）

- `observe_only`：无越线，允许继续观察。
- `pause_rollout`：中度退化，暂停新增批次但不自动回退。
- `rollback_required`：严重退化或连续越线，必须触发自动回退。
- 语义一致性要求：质量门禁、运行门禁与决策包中的 SLO 动作必须一致，否则阻断放量。

## release decision pack 契约（最小）

- 默认输出文件：`release-decision-pack.json`（可通过 `DELIVERY_PIPELINE_DECISION_PACK_FILE` 覆盖）。
- 最小字段：`decision`、`failureCategory`、`rollbackCandidate`、`drillSummary`、`approval`、`correlationKey`、`timestamp`。
- 决策包缺字段或语义不一致时，必须阻断生产灰度。
- 机器可读字段命名保持稳定；下游解析失败应视为阻断条件。

## go-live 失败处理与回退路径（最小）

- `quality` 失败：先修复质量门禁，再重跑 `make delivery-pipeline`。
- `stability` 失败：修复预生产验证或更新证据，禁止跳过预生产直接放量。
- `drillRollback` 失败：先补演练直到达标，再进入 go-live。
- `approvalFreeze` 失败：补齐审批等级或等待冻结窗口结束。
- `auditIntegrity` 失败：补齐人工覆盖审计字段（`actor/reason/timestamp/traceKey`）并确保幂等。

## 预生产 dry-run 语义约束（最小）

- `DRY_RUN_OUTCOME` 仅允许：`allow` / `degrade` / `block`。
- `DRY_RUN_REASON` 与 `DRY_RUN_TRACE_KEY` 必填；缺失视为证据不完整并阻断。
- `allow`：可继续发布；`degrade`：进入保守路径并保留人工确认；`block`：立即终止并回滚准备。
- dry-run 输出必须可追踪到归档目录中的原始证据文件。

## 值班动作模板执行（最小）

- `allow`：`info` 级别，执行 `runbook://runtime-observation`，30 分钟观察窗口。
- `degrade`：`warning` 级别，执行 `runbook://runtime-degrade-recovery`，需 oncall 确认并准备回滚。
- `block`：`critical` 级别，执行 `runbook://runtime-block-rollback`，需 incident commander 审批并执行回滚。

## 人工覆盖审计（最小）

- 触发人工覆盖时，必须记录覆盖人、覆盖前后判定、覆盖原因与时间戳。
- 覆盖记录必须进入审计事件并关联 `correlationKey`，用于后续复盘。
- 人工覆盖字段最小集合：`actor`、`preDecision`、`postDecision`、`reason`、`timestamp`、`traceKey`。
- 对同一 `traceKey` 重放应保持幂等（只记录一次，后续标记 `idempotent`）。

## 验收矩阵（最小）

- `allow`：`make quality-gate` 全部通过，允许推进发布步骤。
- `block`：任一阻断检查失败（如 CRD 漂移），必须先修复再重试。
- `degrade`：演练判定需保守路径，必须进入 `L3` / 人工介入流程后再评估恢复自动化。
- API 契约阻断：`QUALITY_GATE_CATEGORY=api_contract` 或 `QUALITY_GATE_CATEGORY=runtime_production_gating` 时，必须先补齐迁移条件或发布审批再重试。

## SLO 阈值与响应分级

- 推荐初始阈值：`degradeThresholdPercent=60`，`blockThresholdPercent=90`，`sampleWindowMinutes=10`。
- 预算状态分级：`healthy`（<60%）/ `warning`（60%-89%）/ `exhausted`（>=90%）。
- 响应等级映射：`allow -> info`，`degrade -> warning`，`block -> critical`。
- 恢复前置条件：`degrade` 需预算回落到降级阈值以下；`block` 需人工审批与事故复盘通过。

## 风险

- 健康 Revision 误判导致回滚不准确。
- 域级熔断阈值过严导致自愈过度抑制。
- 告警重复风暴导致链路抖动（需依赖幂等窗口）。
- 运行态输入采集失败导致门禁误阻断（需重点关注 GateInputUnavailable 事件）。
- 命名空间预算阈值配置不当导致过度只读阻断（需结合业务基数调参）。
- StatefulSet 受控动作授权链路不完整导致误动作（需同时满足开关、白名单、审批、证据链）。
- StatefulSet 动作失败后未冻结导致重复扰动（需验证 `statefulSetFreezeState=frozen` 与 `statefulSetFreezeUntil`）。
- StatefulSet L2 候选筛选不稳定导致频繁降级 L3（需结合候选窗口参数调优）。
- StatefulSet L2 回滚失败恢复路径异常（需验证 snapshot restore 与冻结联动）。
- 快照容量上限配置过低导致频繁阻断（需结合告警与容量指标调参）。
- 快照恢复耗时过长导致恢复窗口扩大（需按 workload 等级设置恢复超时）。

## 回滚步骤

1. 将策略模式切换为 `L1 + 人工介入`。
2. 关闭 webhook 自动写入，仅保留只读评估与审计。
3. 关闭域级熔断开关，仅保留对象级熔断。
4. 如果仍异常，停用自动写操作，仅保留只读评估与告警。
5. 根据审计记录恢复到最近稳定发布版本。
6. 关闭保守模式预算阻断与白名单尝试权，恢复基础门禁策略。
7. 将 `statefulSetPolicy.controlledActionsEnabled=false` 且 `statefulSetPolicy.readOnlyOnly=true`，回退到只读策略。
8. 清理审批注解 `kube-sentinel.io/statefulset-approved`，避免误触发下一轮自动动作。
9. 将 `statefulSetPolicy.l2RollbackEnabled=false`，回退到 Phase 2（仅 L1 受控 + L3 人工）模式。
10. 将 `snapshotPolicy.enabled=false`（或回退控制器版本）并清理历史快照对象，恢复到保守只读模式。

## 本地与生产 blast radius 约定

- 本地 minikube 或单节点测试环境通常 Pod 基数过小，默认 `blastRadius.maxPodPercentage=10` 会先阻断自动写路径，这属于预期行为。
- 本地 smoke 允许只对当前 `HealingRequest` 临时 patch 更宽松的 `spec.blastRadius.maxPodPercentage`，用于验证 `PendingVerify -> Completed` 成功闭环。
- 不允许把本地 smoke 中的临时阈值写回 chart 默认值、脚本默认值或生产环境配置。
- smoke 完成后应保留默认保守值作为正式环境基线，并在文档或审计中说明本次临时放宽仅用于联调。

## 应急步骤

- 持续失败达到阈值时，立即触发人工介入。
- 对生产命名空间临时启用维护窗口，阻断自动写操作。
- 当证据不足导致频繁 L3 时，暂停 L2 并执行人工版本选择。
- 运行态输入不可用时默认只读阻断，优先恢复输入采集链路后再恢复自动写操作。
- 当命名空间只读阻断持续超过预算阈值窗口，触发人工审批后再启用紧急尝试权。
- StatefulSet 触发自动动作预期时，必须走人工介入流程，不得绕过只读策略。
- 快照创建失败或恢复失败告警触发后，应优先冻结自动写路径并执行人工回退。

## 最小告警集与抑制策略

- `KubeSentinelSLOBudgetWarning`：SLO 预算进入 warning 区间（for `10m`）。
- `KubeSentinelSLOBudgetExhausted`：SLO 预算耗尽并触发阻断（for `5m`）。
- `KubeSentinelQualityGateDegradeStreak`：连续降级超过阈值窗口（for `15m`）。
- `KubeSentinelQualityGateBlockProlonged`：阻断持续超过恢复窗口（for `15m`）。
- 抑制策略：同一对象 `warning` 在 `10m` 内去重；`critical` 仅在状态恢复后允许再次通知。
- 门禁输出需携带抑制元数据：`QUALITY_GATE_ALERT_NOTIFY`、`QUALITY_GATE_ALERT_SUPPRESSED_COUNT`。

## 周期演练与趋势指标（最小）

- 周期演练建议工作日夜间窗口执行（示例：`0 2 * * 1-5`）。
- 关注四个趋势指标：门禁通过率、阻断率、恢复耗时、演练覆盖率。
- 空样本窗口应输出 0 值并保留窗口边界，不得输出非法数值。
- 异常输入（如负恢复耗时）必须失败并阻断统计写入。

## 变更拆分治理门禁（最小）

- 拆分触发阈值：`capability >= 3` 或 `增量需求条目 > 10` 必须拆分。
- 风险域混合约束：`blocking` 与 `operational` 同时出现时，默认阻断；仅在提供例外审批后放行。
- 归档前自检必填：`scopeComplexity`、`riskCoupling`、`reviewability`、`rollbackImpact`。
- 例外审批最小字段：`approver`、`reason`、`timestamp`、`traceKey`。

## 变更拆分治理使用方式（最小）

- 启用门禁：`CHANGE_SPLIT_GOVERNANCE_ENABLED=true`。
- 输入规模：`CHANGE_SPLIT_CAPABILITY_COUNT`、`CHANGE_SPLIT_INCREMENT_ITEMS`。
- 声明拆分计划：`CHANGE_SPLIT_HAS_SPLIT_PLAN=true` 且 `CHANGE_SPLIT_PLAN_REF=<change-a,change-b>`。
- 归档阶段检查：`CHANGE_SPLIT_STAGE=archive` 并提供四项自检字段。
- 幂等提交：`CHANGE_SPLIT_IDEMPOTENCY_FILE` + `CHANGE_SPLIT_SUBMISSION_KEY`。

## 变更拆分治理失败处理（最小）

- `split_required_missing_plan`：补齐拆分计划或提交例外审批。
- `mixed_risk_domains_without_exception`：按风险域拆分或补齐审批。
- `exception_approval_fields_missing`：补齐审批人、原因、时间戳与关联键。
- `pre_archive_checklist_incomplete`：补齐四项自检后再执行归档。

## 变更拆分治理调优计划（最小）

- 每两周回顾一次阈值命中率与误报率；必要时调整阈值。
- 当“拆分后评审时长”连续两个周期无改善时，复核风险域分类标准。
- 例外审批占比超过 20% 时，优先优化拆分模板而非扩大例外范围。
