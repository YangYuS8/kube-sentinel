# V1 Pilot/Cutover 预生产全链路演练记录（样例）

## 演练输入

- 执行时间：2026-03-05T09:30:00Z
- 目标版本：v1.0.0-rc2
- pilot 批次：2
- 当前阶段：pilot_observe
- 目标阶段：cutover_ready
- 关联键：pilot-v1-20260305-0930

## 产出（摘录）

- `DELIVERY_PIPELINE_DECISION=allow`
- `DELIVERY_PIPELINE_PILOT_BATCH=2`
- `DELIVERY_PIPELINE_PILOT_STATE_CURRENT=pilot_observe`
- `DELIVERY_PIPELINE_PILOT_STATE_TARGET=cutover_ready`
- `DELIVERY_PIPELINE_PILOT_STATE_NEXT=cutover_ready`
- `DELIVERY_PIPELINE_SLO_MATRIX_ACTION=observe_only`
- `DELIVERY_PIPELINE_ROLLBACK_EVIDENCE=none`
- `DELIVERY_PIPELINE_GATE_QUALITY_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_STABILITY_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_DRILL_ROLLBACK_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_AUDIT_INTEGRITY_STATUS=pass`

本次实跑证据文件：

- 决策包样例：`docs/evidence/v1-pilot-cutover-decision-pack.sample.json`
- 流水线输出：`.tmp/v1-pilot-cutover-drill/run.out`

## 复盘结论

- pilot 状态迁移满足顺序推进要求，未出现跨阶段跳转。
- 观察窗口门禁、冻结窗口规则、handoff 契约与 SLO 触发矩阵已被自动校验。
- 后续建议：将连续越线回退演练纳入夜间定时作业。

`release-decision-pack.json` 最小字段校验通过：

- decision
- failureCategory
- pilotBatch
- rollbackTarget
- traceKey
- approval
- timestamp
