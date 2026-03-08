# statefulset-reality-validation-and-hardening Proposal

## 为什么

当前仓库已经为 StatefulSet 提供了受控自动动作、L2 健康版本回滚、冻结与快照恢复等主链路语义，但这些能力主要由单元测试和规格约束证明，缺少像 Deployment 那样的真实集群验证闭环。既然项目目标明确要求 Deployment 和 StatefulSet 共同作为核心自愈对象，下一步应优先补上 StatefulSet 在真实 Kubernetes 语义下的可行性验证与必要加固，避免形成“规格完备但真实性不足”的第二个短板。

## 变更内容

- 为 StatefulSet 受控动作与 L2 回滚补齐真实集群语义验证，重点覆盖 ControllerRevision 候选选择、L1 失败升级 L2、L2 失败冻结与 snapshot restore 联动。
- 将一次性真实性检查沉淀为仓库内可重复执行的集成测试，明确启用条件、真实集群前置和预期证据输出。
- 根据真实集群验证结果修正 StatefulSet 适配器、编排状态或诊断语义中的不实假设，确保在无法证明安全时稳定降级。
- 对齐发布/回滚文档与运行时闭环验证说明，把 StatefulSet 真实性验证纳入交付前检查口径。

## 功能 (Capabilities)

### 新增功能

<!-- 无 -->

### 修改功能

- `statefulset-controlled-healing`: 补充真实集群下 L1 受控动作、冻结窗口和失败后只读回退的验证与行为约束。
- `statefulset-healthy-revision-rollback`: 补充 ControllerRevision 候选稳定性、L2 回滚真实性和 snapshot restore 联动的需求细化。
- `runtime-closed-loop-validation`: 把 StatefulSet 真实性验证纳入可重复执行的闭环检查，明确本地或测试集群的验收入口与阻断条件。

## 影响

- 受影响代码：`internal/healing/orchestrator.go`、StatefulSet 适配器/快照恢复相关代码、`internal/healing/*_test.go`、可能新增受环境变量控制的集成测试文件与验证脚本。
- 受影响 API：原则上不新增外部 API 版本，但可能细化现有 `status` 字段在 StatefulSet 冻结、回滚与恢复场景下的使用约定。
- 受影响系统：StatefulSet 自动处置链路、运行时闭环验证、发布前质量门禁与回滚演练说明。
- 风险与依赖：依赖真实 Kubernetes 对 StatefulSet/ControllerRevision 的行为证据；若现有适配器假设与真实集群不一致，需要通过最小增量修正实现并补足集成测试稳定性。
