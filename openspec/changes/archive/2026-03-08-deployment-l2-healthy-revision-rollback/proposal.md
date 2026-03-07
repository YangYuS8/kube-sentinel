# deployment-l2-healthy-revision-rollback Proposal

## 为什么

当前仓库已经具备 Deployment L1 处置、快照校验、StatefulSet 受控回滚和 Deployment L2 相关状态/指标骨架，但 Deployment 在 L1 失败后仍直接标记为 not-allowed-in-mvp，导致 openspec/config.yaml 中要求的 L1 -> L2 -> L3 渐进式处置链路在最常见工作负载上没有闭环。现在应优先补齐这一缺口，因为本地安装、联调和 smoke 回路已经稳定，继续推迟只会让后续发布准备和生产灰度缺少最关键的自动回滚能力。

## 变更内容

- 为 Deployment 补齐 L1 失败后的 L2 健康版本回滚执行路径，不再停留在仅记录 skipped/not-allowed-in-mvp 的占位状态。
- 明确 L2 候选选择、候选不足降级、回滚前门禁校验、执行失败后的快照恢复和 L3 人工介入建议输出。
- 对齐 HealingRequest status、审计事件、指标和失败原因，使 Deployment 的阶段迁移与安全门禁证据可解释、可检索。
- 补齐与上述链路对应的单元测试、失败路径测试、文档和发布/回滚注意事项，为后续灰度启用提供验收基线。

## 功能 (Capabilities)

### 新增功能

### 修改功能

- `deployment-tiered-healing`: 将 Deployment 的 L2 从规格存在但实现跳过，收敛为真实可执行的健康版本回滚流程，并明确 L2 失败后的降级与证据输出。

## 影响

- 受影响代码：`internal/healing/orchestrator.go`、`internal/healing/rollback.go`、Deployment 适配器/快照恢复链路、`internal/observability/*`、相关测试与发布文档。
- 受影响 API：不新增外部 API 版本，但会补充/细化 `HealingRequest.status` 中 Deployment L2 相关阶段、原因和建议字段的使用约定。
- 受影响系统：Deployment 自动处置路径、审计与指标采集、质量门禁和本地 smoke/灰度验证流程。
- 依赖与风险：需要复用现有快照与健康候选判定逻辑，并确保新回滚动作仍满足幂等性、爆炸半径、熔断和维护窗口约束。
