## ADDED Requirements

### 需求:StatefulSet Phase 2 验收矩阵

系统必须提供 StatefulSet Phase 2 的验收矩阵，覆盖授权通过、授权失败、动作失败回退、冻结阻断与回滚恢复场景。

#### 场景: Phase 2 全路径验收

- **当** 执行 Phase 2 演练
- **那么** 系统必须输出各场景通过结果与证据链完整性

### 需求:受控动作回归门禁

系统必须将 StatefulSet Phase 2 纳入 CI 验证，确保核心失败路径与边界值测试稳定通过。

#### 场景: CI 验证失败阻断

- **当** Phase 2 失败路径或边界测试未通过
- **那么** 系统必须阻断交付

## MODIFIED Requirements

### 需求:运行时闭环验收基线

系统必须在既有闭环基线上增加 StatefulSet 条件可写路径断言，并验证默认只读、授权可写、失败回退三态可复现。

#### 场景: 三态行为断言

- **当** 执行闭环验收脚本
- **那么** 系统必须验证 StatefulSet 在三态之间的转换符合策略定义

## REMOVED Requirements
