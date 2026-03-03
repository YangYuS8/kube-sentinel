## ADDED Requirements

### 需求:StatefulSet Phase 3 验收矩阵
系统必须提供 StatefulSet Phase 3 验收矩阵，覆盖 L1 失败转 L2、L2 成功、L2 失败恢复、冻结阻断与 L3 降级。

#### 场景: Phase 3 全路径验收
- **当** 执行 StatefulSet Phase 3 演练
- **那么** 系统必须输出各路径通过结果与证据链完整性

### 需求:StatefulSet L2 回归门禁
系统必须将 StatefulSet L2 纳入 CI 回归门禁，且失败路径与边界值测试未通过时必须阻断交付。

#### 场景: L2 失败路径阻断
- **当** L2 回滚失败恢复或冻结路径测试未通过
- **那么** 系统必须阻断交付

## MODIFIED Requirements

### 需求:运行时闭环验收基线
系统必须在既有闭环基线上增加 StatefulSet L2 路径断言，并验证 L1→L2→L3 状态转换可复现。

#### 场景: 三阶段状态断言
- **当** 执行闭环验收脚本
- **那么** 系统必须验证 StatefulSet 三阶段转换符合策略定义

## REMOVED Requirements