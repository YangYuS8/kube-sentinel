# V1 Go-Live 预生产全链路演练记录（样例）

## 演练输入

- 执行时间：2026-03-05T00:30:00Z
- 目标版本：v1.0.0-rc1
- 关联键：drill-v1-20260305-0030
- 预生产状态：allow
- 演练阈值：successRate >= 0.95，rollbackP95Ms <= 300000

## 产出（摘录）

- `DELIVERY_PIPELINE_DECISION=allow`
- `DELIVERY_PIPELINE_FAILURE_CATEGORY=none`
- `DELIVERY_PIPELINE_GATE_QUALITY_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_STABILITY_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_DRILL_ROLLBACK_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=pass`
- `DELIVERY_PIPELINE_GATE_AUDIT_INTEGRITY_STATUS=pass`

本次实跑证据文件：

- 决策包样例：`docs/evidence/v1-go-live-decision-pack.sample.json`
- 流水线输出：`.tmp/v1-go-live-drill/run.out`

`release-decision-pack.json` 最小字段校验通过：

- decision
- failureCategory
- rollbackCandidate
- drillSummary
- approval
- correlationKey
- timestamp

## 复盘结论

- go-live 决策链路满足“任一失败即 block、全部通过才 allow”的 V1 语义。
- 预生产证据时效、演练阈值、审批等级、冻结窗口、覆盖审计均已被自动约束。
- 后续改进建议：把 decision pack 聚合进夜间趋势报告，增加窗口化稳定性对比。
