# 主线A最小验收清单

## 输入条件

- Alertmanager Webhook 可访问：`/alertmanager/webhook`
- 目标 Deployment 存在且可识别
- HealingRequest CRD 已安装
- 统一质量门禁已执行：`make quality-gate`
- 如需校验预提交与 CI 语义一致性，需同时提供 `PRECOMMIT_GATE_OUTCOME` 与 `CI_GATE_OUTCOME`
- 如需校验门禁与 SLO 治理语义一致性，可提供 `SLO_GOVERNANCE_OUTCOME`

## 预期行为

- 统一质量门禁输出可解析结论：`QUALITY_GATE_RESULT` / `QUALITY_GATE_CATEGORY` / `QUALITY_GATE_REASON`
- 统一质量门禁输出 SLO 关联字段：`QUALITY_GATE_SLO_ACTION_LEVEL` / `QUALITY_GATE_SLO_BUDGET_STATUS` / `QUALITY_GATE_INCIDENT_LEVEL`
- 合法告警可创建/更新 `HealingRequest`
- 非 Deployment 事件仅只读拒绝，不触发写操作
- 无健康 Revision 时阶段进入 `L3`
- 熔断触发后自动写操作被阻断
- `status/event/metric/audit` 可通过 `correlationKey` 关联
- 自动写动作前必须生成 `status.lastSnapshotId`
- 回滚失败时必须记录 `status.snapshotRestoreResult`（`success` 或 `failed`）
- 演练脚本必须覆盖 `allow` / `block` / `degrade` 三类门禁结果
- 演练脚本必须输出 incident 证据：级别、恢复条件、runbook 标识
- 演练脚本必须输出灰度闭环证据：`rollout.canaryStable`、`rollout.rollbackHit`、`rollout.tuningApproved`、`rollout.recoveryObserved`
- 演练脚本必须校验复盘字段：`postmortem.breachReason`、`postmortem.mitigationAction`、`postmortem.thresholdDecision`、`postmortem.observationPlan`

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
