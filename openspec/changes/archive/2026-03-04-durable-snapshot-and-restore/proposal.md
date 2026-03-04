## 为什么

当前自愈链路在执行动作前已调用快照接口，但默认实现仍为内存占位，无法提供可持久、可审计、可恢复验证的“完整备份”能力。在 `config.yaml` 明确要求“所有自愈动作前必须通过回滚性校验（完整 Snapshot）”的前提下，需要将快照能力提升为生产可用基线，并同时覆盖 Deployment 与 StatefulSet 的 L1/L2 路径。

## 变更内容

- 引入持久化快照能力：为每次自动处置生成可追踪的快照记录（包含目标对象标识、版本证据、校验信息、恢复元数据）。
- 将处置执行与快照强绑定：快照创建失败时禁止写操作并降级；回滚失败时按快照执行恢复并输出可诊断证据。
- 统一 Deployment/StatefulSet 的回滚性校验语义与失败原因码，保证阶段升级可解释。
- 增加快照生命周期治理（保留窗口、清理策略、幂等键），避免无限增长与重复副作用。
- 扩展可观测与交付门禁：新增快照创建/恢复成功率与失败率指标、最小告警与演练断言。

## 功能 (Capabilities)

### 新增功能
- `durable-snapshot-and-restore`: 定义持久化快照、恢复执行、生命周期治理与证据可观测的统一规范。

### 修改功能
- `deployment-healing-orchestration`: 增加 Deployment 自动动作前的持久快照校验与失败降级语义。
- `statefulset-controlled-healing`: 增加 StatefulSet L1/L2 前置快照与失败恢复约束。
- `healthy-revision-rollback`: 增加候选回滚执行与快照恢复的证据链要求。
- `runtime-closed-loop-validation`: 增加快照创建失败、恢复失败、幂等阻断的验收路径。
- `runtime-production-hardening`: 增加快照相关发布阈值、阻断指标与灰度策略。

## 影响

- 代码：`internal/healing`（snapshot/orchestrator/adapter）、`internal/observability`、`internal/controllers`。
- API/CRD：可能需要补充快照策略参数与状态字段（保留窗口、失败原因、恢复结果）。
- 交付物：Helm values/schema、告警规则、演练脚本、发布与回滚文档。
- 运维：新增快照存储开销与治理策略，需在灰度阶段观测性能与容量影响。
