## 为什么

最近一次本地 minikube 闭环联调暴露出三个会直接阻断首发版本可重复交付的问题：仓库内生成的 HealingRequest CRD 清单不可直接安装、HealingRequest 在 PendingVerify 与人工变更并发时容易出现 resourceVersion 冲突，以及请求从阻断态转为成功态后仍残留过期的阻断字段。现在修复这些问题，是为了把“能通过单元测试”提升为“能在真实集群中稳定安装、运行、复测”。

## 变更内容

- 修复 HealingRequest CRD 生成链路，使仓库中的生成物包含正确的 group、version、resource name，并可直接安装到新集群。
- 收敛 HealingRequest reconcile 的时间推进与状态持久化策略，避免 PendingVerify、门禁判定和用户 patch 并发时持续出现对象版本冲突。
- 为 Deployment 首发 L1 状态输出建立成功态归一化规则，确保请求从 blocked 或 pending 转入 completed 后，不再残留过期的阻断原因、影子动作或失败字段。
- 增强 CRD 一致性检查与闭环 smoke 验证标准，使“生成物一致”升级为“生成物正确且可安装”。
- 补充联调与发布文档，明确小规模集群下 blast radius 门禁对闭环验证的影响与验证路径。

## 功能 (Capabilities)

### 新增功能

- 无

### 修改功能

- `api-contract-governance`: 修正 CRD 生成与校验要求，并补充状态语义在成功态、阻断态之间的收敛约束。
- `deployment-healing-orchestration`: 修正 Deployment 首发 L1 在 PendingVerify、门禁判定和成功完成后的状态推进与最终状态语义。

## 影响

- API 包元数据与 CRD 生成产物
- HealingRequest controller 与 orchestrator 的状态持久化逻辑
- CRD 一致性测试、端到端 smoke 验证与联调文档
- 首发 Deployment L1 闭环在真实集群中的可安装性、可重复性与可诊断性
