## 为什么

当前闭环中 Deployment 路径已具备回滚能力，但“显式 L1→L2→L3 分层状态机”尚未完整建模，导致策略升级可解释性、失败证据链一致性与演练可验证性仍有缺口。为满足 `config.yaml` 的多级联动与可回滚优先目标，需要把 Deployment 路径提升为与 StatefulSet 同等级的阶段化处置体系。

## 变更内容

- 为 Deployment 引入显式分层状态机：L1（最小影响动作）→L2（健康版本回滚）→L3（人工介入）。
- 在 L1/L2 统一接入持久快照与恢复语义，确保执行前可回滚性校验、执行失败可恢复。
- 增强阶段升级可解释性：记录升级触发条件、失败证据、阻断原因码与下一步建议。
- 强化 Deployment 分层路径的幂等窗口控制、门禁一致性与冻结/降级语义。
- 扩展可观测与交付门禁：补充 Deployment 分层指标、最小告警与闭环演练断言。

## 功能 (Capabilities)

### 新增功能
- `deployment-tiered-healing`: 定义 Deployment 显式 L1/L2/L3 分层编排语义、升级条件、降级与冻结证据。

### 修改功能
- `deployment-healing-orchestration`: 将 Deployment 路径升级为显式分层状态机并补齐幂等阻断证据。
- `healthy-revision-rollback`: 明确 Deployment L2 回滚候选筛选、依赖校验、恢复证据输出。
- `runtime-closed-loop-validation`: 增加 Deployment 三阶段验收矩阵与失败路径回归门禁。
- `runtime-production-hardening`: 增加 Deployment 分层灰度发布阈值、阻断指标与回退策略。
- `tiered-circuit-breaking`: 细化 Deployment 分层动作与对象/域级熔断联动规则。

## 影响

- 代码：`internal/healing/orchestrator.go`、`internal/healing/rollback.go`、`internal/observability/*`、`internal/controllers/*`。
- API/CRD：可能扩展 Deployment 分层策略参数与状态字段（阶段决策、失败原因、建议动作）。
- 交付物：Helm values/schema、告警规则、演练脚本、发布与回滚文档。
- 运维：新增 Deployment 分层路径指标与告警，需要灰度阶段持续观测阈值越线情况。
