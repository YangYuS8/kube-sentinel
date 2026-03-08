# 主线A最小验收清单

## 输入条件

- Alertmanager Webhook 可访问：`/alertmanager/webhook`
- 目标 Deployment 存在且可识别
- HealingRequest CRD 已安装
- 统一质量门禁已执行：`make quality-gate`
- 如需执行 StatefulSet 真实性验证，必须显式设置 `KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY=true`；默认未启用时允许脚本与集成测试清晰跳过
- 如需校验预提交与 CI 语义一致性，需同时提供 `PRECOMMIT_GATE_OUTCOME` 与 `CI_GATE_OUTCOME`
- 如需校验门禁与 SLO 治理语义一致性，可提供 `SLO_GOVERNANCE_OUTCOME`

## 预期行为

- 统一质量门禁输出可解析结论：`QUALITY_GATE_RESULT` / `QUALITY_GATE_CATEGORY` / `QUALITY_GATE_REASON`
- 统一质量门禁输出 SLO 关联字段：`QUALITY_GATE_SLO_ACTION_LEVEL` / `QUALITY_GATE_SLO_BUDGET_STATUS` / `QUALITY_GATE_INCIDENT_LEVEL`
- 统一质量门禁输出 API 契约证据：`QUALITY_GATE_API_COMPATIBILITY_CLASS` / `QUALITY_GATE_API_AFFECTED_FIELDS` / `QUALITY_GATE_API_MIGRATION_PLAN` / `QUALITY_GATE_API_RISK_LEVEL`
- 统一质量门禁输出发布绑定字段：`QUALITY_GATE_RELEASE_DECISION` / `QUALITY_GATE_VERSION_BUMP_WINDOW`
- 合法告警可创建/更新 `HealingRequest`
- 非 Deployment 事件仅只读拒绝，不触发写操作
- 无健康 Revision 时阶段进入 `L3`
- 熔断触发后自动写操作被阻断
- `status/event/metric/audit` 可通过 `correlationKey` 关联
- 自动写动作前必须生成 `status.lastSnapshotId`
- 回滚失败时必须记录 `status.snapshotRestoreResult`（`success` 或 `failed`）
- 本地 smoke 脚本必须覆盖默认 `block` 与单次放宽后的 `allow` 路径；`degrade` 语义继续由单元测试与发布门禁校验覆盖
- StatefulSet 真实性验证入口必须区分 Deployment smoke 与 StatefulSet reality：`bash ./scripts/drill-statefulset-reality.sh default` 仅在显式启用后运行，输出至少包含 `STATEFULSET_REALITY_CONTEXT`、`STATEFULSET_REALITY_TEST_PATTERN`、`STATEFULSET_REALITY_RESULT`
- StatefulSet 真实性验证通过标准：必须覆盖历史候选存在、历史候选缺失，以及 L2 回滚失败后的冻结/快照恢复证据
- 演练脚本必须输出 incident 证据：级别、恢复条件、runbook 标识
- 演练脚本必须输出灰度闭环证据：`rollout.canaryStable`、`rollout.rollbackHit`、`rollout.tuningApproved`、`rollout.recoveryObserved`
- 演练脚本必须校验复盘字段：`postmortem.breachReason`、`postmortem.mitigationAction`、`postmortem.thresholdDecision`、`postmortem.observationPlan`
- 发布就绪摘要必须包含：`actionType/riskLevel/strategyMode/circuitTier/rollbackCandidate/openIncidents/recentDrillScore`
- 值班模板映射必须覆盖：`allow/degrade/block`，并输出对应 runbook 与审批触发点
- 人工覆盖触发时必须输出审计证据：`operatorOverride.by/from/to/reason/at`
- pilot/cutover 必须输出状态机字段：`DELIVERY_PIPELINE_PILOT_STATE_CURRENT`、`DELIVERY_PIPELINE_PILOT_STATE_TARGET`、`DELIVERY_PIPELINE_PILOT_STATE_NEXT`
- pilot/cutover 必须输出批次与回退证据：`DELIVERY_PIPELINE_PILOT_BATCH`、`DELIVERY_PIPELINE_ROLLBACK_EVIDENCE`
- cutover 决策包必须包含最小字段：`decision/failureCategory/pilotBatch/rollbackTarget/traceKey/approvalLevel/timestamp`
- 值班交接字段必须齐备：`handoffOwner/approvalLevel/traceKey/rollbackCommandRef/handoffTimestamp`

## 失败路径

- 缺少关键 labels（`workload_kind/namespace/name`）返回可诊断错误
- 重复事件在幂等窗口内被抑制
- 门禁命中（维护窗口/速率限制/爆炸半径）时仅只读评估+告警
- 证据不足时禁止 L2 回滚并输出人工介入建议
- 快照创建失败时必须阻断写操作并输出 `snapshot-failed`
- 快照恢复失败时必须进入冻结并输出人工介入建议
- 预提交与 CI 门禁语义不一致时必须阻断验收
- 门禁语义与 SLO 治理语义不一致时必须阻断验收
- 恢复条件未满足时即使检查项通过也必须阻断放量（`QUALITY_GATE_RECOVERY_READY=false`）
- 本地 smoke 中对 `blastRadius` 的放宽必须仅作用于当前 `HealingRequest`，不得回写 chart 默认值或生产配置
- 未显式启用 StatefulSet 真实性验证、缺失 `kubectl/go`、或当前上下文不是 `minikube` 时，`scripts/drill-statefulset-reality.sh` 必须输出 `STATEFULSET_REALITY_RESULT=skip` 与可诊断原因，禁止静默成功
- 兼容性分类非法、迁移路径缺失或高风险未审批时必须阻断放量
- API/CRD/Helm 任一约束未同步时必须阻断 CI 与质量门禁
- 非法状态迁移（如 `pilot_prepare -> cutover_done`）必须阻断并输出 `invalid_stage_transition`
- pilot 观察窗口未完成时禁止 cutover
- SLO 动作语义不一致（质量门禁/运行门禁/决策包）时必须阻断并输出 `slo_threshold_contract_mismatch`
- 命中自动回退触发条件时必须输出 `cutover_auto_rollback`
