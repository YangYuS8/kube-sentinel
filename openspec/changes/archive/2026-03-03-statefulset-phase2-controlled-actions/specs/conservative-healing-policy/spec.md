## ADDED Requirements

### 需求:StatefulSet 条件可写策略

系统必须支持 StatefulSet 的 `conditional-writable` 策略级别，并定义从只读到可写的最小授权集合。

#### 场景: 条件可写策略生效

- **当** StatefulSet 策略级别被设置为 `conditional-writable`
- **那么** 系统必须仅在授权门禁通过时执行受限自动动作

### 需求:失败冻结策略

系统必须在 StatefulSet 受控动作失败后进入冻结期，冻结期间禁止任何自动写操作。

#### 场景: 冻结期阻断

- **当** StatefulSet 处于失败冻结期
- **那么** 系统必须阻断自动写操作并输出冻结剩余时间

## MODIFIED Requirements

### 需求:StatefulSet 默认只读策略

系统必须将 StatefulSet 的默认策略保持为只读；即使启用 Phase 2 代码路径，未显式授权时仍禁止自动写操作。

#### 场景: 默认策略未被覆盖

- **当** 未提供 StatefulSet 条件可写授权配置
- **那么** 系统必须继续执行只读阻断

### 需求:保守执行总原则

系统必须以“绝不误动作”为第一优先级；在 Phase 2 下若证据链、授权门禁或冻结策略任一不满足，必须立即降级人工介入。

#### 场景: Phase 2 保护降级

- **当** StatefulSet 条件可写路径发生任一保护条件失败
- **那么** 系统必须停止自动写操作并回退只读

## REMOVED Requirements
