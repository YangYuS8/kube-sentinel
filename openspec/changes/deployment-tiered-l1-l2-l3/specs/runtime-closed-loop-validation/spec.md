## ADDED Requirements

### 需求:Deployment 三阶段验收矩阵

系统必须提供 Deployment L1/L2/L3 三阶段验收矩阵，覆盖 L1 成功、L1 失败升级 L2、L2 失败降级 L3 与恢复路径。

#### 场景: Deployment 三阶段全路径验收

- **当** 执行 Deployment 分层闭环演练
- **那么** 系统必须输出三阶段路径通过结果与证据链完整性

## MODIFIED Requirements

### 需求:运行时闭环验收基线

系统必须在既有闭环基线上增加 StatefulSet L2 路径断言，并验证 L1→L2→L3 状态转换可复现；同时必须校验快照创建、回滚失败恢复与幂等阻断证据可复现；Deployment 分层路径必须纳入同等断言。

#### 场景: 三阶段状态断言

- **当** 执行闭环验收脚本
- **那么** 系统必须验证 StatefulSet 三阶段转换符合策略定义

#### 场景: 快照证据断言

- **当** 执行闭环验收脚本覆盖失败路径
- **那么** 系统必须验证快照与恢复证据链完整可检索

#### 场景: Deployment 分层断言

- **当** 执行 Deployment 分层闭环演练
- **那么** 系统必须验证 Deployment L1/L2/L3 迁移与阻断语义符合策略定义

## REMOVED Requirements
