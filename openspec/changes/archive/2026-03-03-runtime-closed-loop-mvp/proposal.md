## 为什么

当前 Kube-Sentinel 已具备策略与门禁骨架，但尚未形成可验证的运行时闭环（真实输入、状态流转、观测输出）。在进入更高复杂度优化前，需要先完成主线A的最小端到端链路，验证系统“可运行、可观测、可解释”。

## 变更内容

- 增加 Alertmanager Webhook 到自愈请求（HealingRequest）的运行时接入链路，建立统一事件入口。
- 补齐 Reconciler 端到端状态流转约定：门禁判定、策略阶段推进、失败降级与人工介入。
- 增加 MVP 级运行验证规范：仅 Deployment 可写、无健康 Revision 降级 L3、双层熔断有效阻断。
- 增加最小观测闭环：状态字段、K8s Event、指标与审计记录必须互相可关联。

## 功能 (Capabilities)

### 新增功能
- `alertmanager-webhook-ingestion`: 定义 Alertmanager 告警事件进入系统并映射为 HealingRequest 的契约与失败处理。
- `runtime-closed-loop-validation`: 定义主线A运行时闭环的 MVP 验收场景与判定标准。

### 修改功能
- `deployment-healing-orchestration`: 将现有编排规范扩展为“真实事件驱动 + 状态可追踪”的运行时行为。
- `healthy-revision-rollback`: 明确运行态下健康 Revision 证据不足时的处理与降级输出。
- `tiered-circuit-breaking`: 明确对象级/域级熔断在真实事件洪峰下的优先级与恢复可观测行为。

## 影响

- 受影响代码：事件入口层、控制器编排层、策略/熔断模块、可观测组件。
- 受影响 API：HealingRequest 相关 spec/status 字段与事件映射规则。
- 受影响系统：Alertmanager Webhook 对接、Prometheus 指标与告警路由。
- 交付影响：需要增加运行时闭环测试（模拟告警、失败注入、状态断言）与 CI 检查项。
