## ADDED Requirements

### 需求:StatefulSet L1-L2-L3 阶段化编排
系统必须在统一编排状态机中支持 StatefulSet 的 L1→L2→L3 渐进式处置路径。

#### 场景: L1 失败后升级
- **当** StatefulSet L1 失败且满足升级条件
- **那么** 系统必须升级执行 L2 回滚判定

#### 场景: L2 无候选降级
- **当** StatefulSet L2 未找到健康候选或证据不足
- **那么** 系统必须降级到 L3 人工介入

## MODIFIED Requirements

### 需求:编排链路幂等执行
系统必须保证多工作负载下同一告警重复触发不会导致重复副作用；在 StatefulSet L2 模式下，必须对同一窗口内重复回滚执行进行幂等阻断。

#### 场景: StatefulSet L2 重复触发
- **当** StatefulSet 在同一幂等窗口重复触发 L2 回滚
- **那么** 系统必须拒绝重复回滚并输出幂等阻断证据

## REMOVED Requirements