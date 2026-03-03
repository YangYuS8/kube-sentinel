## 为什么

当前链路已覆盖 StatefulSet 的 Phase 1（只读）与 Phase 2（受控 L1 重启动作），但尚未补齐 L2“健康版本回滚”能力，导致在 L1 无效时只能直接人工介入。为对齐 `openspec/config.yaml` 的“L1→L2→L3 渐进式处置”目标，需要为 StatefulSet 增加可审计、可回退、强门禁的 L2 自动回滚能力。

## 变更内容

- 新增 StatefulSet L2 健康版本回滚能力：支持在证据充分时自动回滚到可验证健康版本。
- 执行前强制快照与依赖完整性校验；校验失败或证据不足时直接降级 L3。
- 保持保守默认：仅在 Phase 2/3 开关、审批与授权链完整时允许进入 StatefulSet L2。
- 为 L2 增加失败回退与冻结保护，避免重复扰动；失败后回退只读并升级人工介入。
- 增强可观测：输出 StatefulSet L2 候选筛选证据、回滚结果与失败原因指标。

## 功能 (Capabilities)

### 新增功能
- `statefulset-healthy-revision-rollback`: 定义 StatefulSet L2 健康版本候选筛选、回滚执行、失败恢复与降级规则。

### 修改功能
- `healthy-revision-rollback`: 从仅面向 Deployment 扩展为支持 StatefulSet 的通用候选语义与证据输出。
- `statefulset-controlled-healing`: 增加 L1 失败后进入 L2 的升级条件、冻结协同与降级触发点。
- `deployment-healing-orchestration`: 扩展统一编排状态机，支持 StatefulSet 的 L1→L2 升级路径与幂等约束。
- `runtime-closed-loop-validation`: 新增 StatefulSet L2 正向/失败/降级验收矩阵与回归门禁。
- `runtime-production-hardening`: 新增 StatefulSet L2 灰度发布阈值、回滚开关与发布阻断要求。

## 影响

- 代码影响：`internal/healing` 编排与适配器、`internal/safety` 门禁协同、`internal/observability` 指标与审计。
- API/CRD 影响：`HealingRequest.status` 可能新增 L2 候选、执行结果、降级建议等状态字段（保持向后兼容）。
- 配置影响：Helm values/schema 需增加 StatefulSet L2 开关、候选窗口、失败冻结与灰度阈值配置。
- 交付影响：需补充脚本与文档，确保 `go test/race/vet/lint` 与 OpenSpec 严格校验通过。