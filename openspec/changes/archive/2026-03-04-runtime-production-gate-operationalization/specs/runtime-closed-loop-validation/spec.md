## ADDED Requirements

### 需求:门禁阻断与降级演练矩阵
系统必须在闭环演练中覆盖 `allow`、`block`、`degrade` 三类门禁结果，并验证每类结果的证据输出完整性。

#### 场景: 三类门禁结果全覆盖
- **当** 执行预生产门禁演练
- **那么** 系统必须输出 allow/block/degrade 三类路径的断言结果

## MODIFIED Requirements

### 需求:Deployment 三阶段验收矩阵
系统必须提供 Deployment L1/L2/L3 三阶段验收矩阵，覆盖 L1 成功、L1 失败升级 L2、L2 失败降级 L3 与恢复路径；并必须增加灰度门禁阻断与自动降级断言。

#### 场景: Deployment 三阶段全路径验收
- **当** 执行 Deployment 分层闭环演练
- **那么** 系统必须输出三阶段路径通过结果与证据链完整性

#### 场景: Deployment 门禁阻断断言
- **当** 执行 Deployment 灰度门禁演练
- **那么** 系统必须验证门禁阻断原因码与降级动作语义一致

## REMOVED Requirements
