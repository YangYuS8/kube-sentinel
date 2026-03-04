## 为什么

当前 Kube-Sentinel 已具备 Deployment/StatefulSet 分层处置、熔断与闭环验收能力，但“指标可观测”尚未完全闭合为“生产可执行门禁”。上线决策仍依赖人工解读与临时判断，缺少统一的阻断原因码、自动降级策略与可重复的发布验证路径。为了达成首个可上线版本，需要把发布门禁从能力点提升为运营化流程。

## 变更内容

- 建立统一的生产门禁决策模型：将分层成功率、降级率、阻断率、熔断信号与快照恢复质量聚合为明确的 `allow/block/degrade` 判定。
- 明确门禁优先级：维护窗口/安全门禁 > 熔断 > 发布门禁阈值，避免策略冲突导致误放行。
- 固化降级动作：当门禁越线时自动切回保守模式（只读评估 + 告警 + 人工介入建议）。
- 统一证据输出：要求每次门禁判定输出结构化原因码、阈值快照、关联指标与回退建议。
- 强化上线验证：将“预生产演练 + 小流量灰度 + 失败回退演练”收敛为可重复的验收矩阵与交付门禁。

## 功能 (Capabilities)

### 新增功能
- `runtime-production-gating`: 定义生产门禁的统一判定模型、优先级顺序、降级动作与证据输出契约。

### 修改功能
- `runtime-production-hardening`: 从“指标建议”升级为“可执行阻断与回退策略”。
- `runtime-closed-loop-validation`: 新增预生产与灰度阶段的门禁验收矩阵与阻断断言。
- `tiered-circuit-breaking`: 明确熔断与发布门禁并发命中时的优先级与动作语义。
- `deployment-tiered-healing`: 约束 Deployment 三阶段路径对生产门禁的输入与输出证据字段。
- `healthy-revision-rollback`: 约束 L2 回滚结果在门禁评估中的权重与失败降级语义。

## 影响

- 代码：`internal/healing/orchestrator.go`、`internal/observability/metrics.go`、`internal/safety/*`、`scripts/drill-runtime-closed-loop.sh`。
- API/CRD：可能扩展门禁策略配置字段与状态证据字段（判定结果、原因码、阈值快照、回退建议）。
- 交付物：`charts/kube-sentinel/values.yaml` 与 `values.schema.json` 需要同步门禁参数约束。
- 运维：发布流程新增“门禁判定报告”与“越线自动降级”检查点。