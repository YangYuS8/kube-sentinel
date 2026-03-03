## 为什么

StatefulSet 已完成 Phase 1 的保守只读接入，但仍全部依赖人工处置，距离“Deployment/StatefulSet 核心对象都具备可控自愈能力”的目标还差关键一步。当前最合适的下一步是引入默认关闭、强门禁保护的 Phase 2 受控自动动作，以在不牺牲安全性的前提下提升恢复效率。

## 变更内容

- 新增 StatefulSet Phase 2 受控动作能力：在显式授权、白名单和审批条件同时满足时，允许单对象、单次、可回滚的自动动作。
- 默认保持 StatefulSet 只读；未满足授权条件时必须继续只读阻断并给出可检索原因。
- 引入 StatefulSet 专属安全门禁（有序副本健康、PDB 约束、存储依赖完整性、角色安全约束）。
- 建立动作失败即回退只读的保护机制，并补充可观测与验收指标门槛。

## 功能 (Capabilities)

### 新增功能

- `statefulset-controlled-healing`: 定义 StatefulSet 受控自动动作的授权条件、执行边界、回退策略与验收标准。

### 修改功能

- `statefulset-conservative-onboarding`: 从“纯只读”扩展为“默认只读 + 条件可写”的阶段化策略。
- `conservative-healing-policy`: 增补 StatefulSet 受控动作开关、审批与失败回退只读规则。
- `deployment-healing-orchestration`: 抽象多工作负载动作编排入口，确保 Deployment/StatefulSet 一致的策略框架与幂等语义。
- `runtime-production-hardening`: 增补 Phase 2 的灰度发布、回滚、观测与质量门禁要求。
- `runtime-closed-loop-validation`: 增补 StatefulSet Phase 2 的正向、失败、回退与边界场景验收矩阵。

## 影响

- 代码影响：`internal/healing` 编排与适配器、`internal/safety` 门禁、`internal/observability`、`api/v1alpha1`、演练脚本与测试。
- API/CRD 影响：可能新增 StatefulSet 受控动作开关、审批状态与执行结果字段（保持向后兼容）。
- 运维影响：新增按 workloadKind/actionType/decision 分类的指标与告警规则。
- 发布影响：默认关闭 Phase 2 写动作；需通过灰度与验收门槛后逐步启用。
