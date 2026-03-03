## ADDED Requirements

### 需求:L1 失败后进入 L2 的升级条件
系统必须在 StatefulSet L1 受控动作失败且授权条件仍满足时，才允许升级进入 L2 健康版本回滚判定。

#### 场景: L1 失败触发 L2 判定
- **当** StatefulSet L1 动作失败且未命中冻结阻断
- **那么** 系统必须进入 L2 候选筛选流程

## MODIFIED Requirements

### 需求:失败回退只读
系统在 StatefulSet 自动动作失败后，必须立即回退到只读模式并冻结后续自动重试，直到人工解锁；当 L2 回滚失败时同样必须执行该规则。

#### 场景: L2 失败回退只读
- **当** StatefulSet L2 回滚执行失败
- **那么** 系统必须进入只读冻结状态并要求人工介入

## REMOVED Requirements