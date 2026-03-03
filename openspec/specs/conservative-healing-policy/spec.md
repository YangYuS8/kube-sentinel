## ADDED Requirements

### 需求:StatefulSet 默认只读策略

系统必须将 StatefulSet 定义为默认只读策略对象，除非显式满足后续阶段开关与审批条件，否则禁止自动写操作。

#### 场景: 默认策略生效

- **当** 未启用 StatefulSet 自动动作开关
- **那么** 系统必须对 StatefulSet 执行只读评估并阻断所有自动写操作

## MODIFIED Requirements

### 需求:保守执行总原则

系统必须以“绝不误动作”为第一优先级；该原则对 StatefulSet 与 Deployment 等价生效，且 StatefulSet 在当前阶段必须优先执行只读阻断。

#### 场景: StatefulSet 证据不足或未授权

- **当** StatefulSet 不满足证据链或未通过阶段授权
- **那么** 系统必须进入只读阻断并输出人工介入建议

### 需求:影子执行说明

系统在阻断自动动作时必须输出“本应执行动作 + 阻断原因 + 证据摘要”的影子执行说明；对 StatefulSet 必须附加“当前阶段仅只读”原因码。

#### 场景: StatefulSet 阻断说明

- **当** StatefulSet 命中自动动作触发条件
- **那么** 系统必须输出包含阶段约束原因的影子执行说明

## REMOVED Requirements
