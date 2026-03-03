## 为什么

当前 Kube-Sentinel 缺少可落地的 v1alpha1 范围定义，导致自愈策略、回滚判定与熔断治理边界不清晰，难以稳定推进实现与评审。需要先收敛第一阶段能力：限定对象、明确回滚依据、定义风险控制分级。

## 变更内容

- 第一阶段（v1alpha1）仅支持 Deployment 作为自愈对象；在架构上预留 Workload 适配接口，便于后续扩展 StatefulSet。
- 将 L2 镜像回滚目标明确为“最近一次健康运行的版本”，通过 Deployment Revision 历史进行选择与验证。
- 引入双层熔断机制（分级治理）：对象级熔断 + 命名空间/全局级熔断，防止局部失败扩散为系统级抖动。
- 明确上述行为在 API 状态、审计记录、告警与任务验收中的表现形式，确保可观测、可解释、可回滚。

## 功能 (Capabilities)

### 新增功能
- `deployment-healing-orchestration`: v1alpha1 的 Deployment 自愈编排主链路与 Workload 适配接口。
- `healthy-revision-rollback`: 基于 Revision 历史识别“最近健康版本”并执行可验证回滚。
- `tiered-circuit-breaking`: 双层熔断策略、触发条件、恢复条件与治理优先级。

### 修改功能
- （无）

## 影响

- 受影响代码：控制器编排层、策略执行引擎、安全门禁模块、可观测性与审计模块。
- 受影响 API：v1alpha1 CRD 的 spec/status 字段需要补充回滚与熔断相关表达。
- 受影响系统：Prometheus 指标与 Alertmanager 告警路由需新增熔断与回滚维度。
- 交付流程影响：CI 需增加 Revision 相关测试和熔断边界测试，保障质量门禁可执行。
