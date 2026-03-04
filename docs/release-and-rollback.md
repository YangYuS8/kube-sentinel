# 发布与回滚说明

## 灰度启用策略

0. 合并前必须执行统一交付门禁：`make quality-gate`。
1. 首先在低风险命名空间启用 webhook 接入与对象级熔断。
2. 校验运行参数为声明式配置驱动（幂等窗口、限频、爆炸半径、熔断阈值）。
3. 执行主线A演练脚本，确认三项强断言通过。
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
