## ADDED Requirements

### 需求:StatefulSet 保守只读接入

系统必须支持将 StatefulSet 事件纳入保守评估链路，并在默认策略下禁止自动写操作。

#### 场景: StatefulSet 事件进入只读链路

- **当** 接收到目标为 StatefulSet 的告警事件
- **那么** 系统必须执行门禁评估与证据计算，但禁止执行 L1/L2 写操作

### 需求:StatefulSet 影子执行说明

系统在阻断 StatefulSet 自动动作时必须输出影子执行说明，包含拟执行动作、阻断原因与人工介入建议。

#### 场景: 只读阻断输出影子执行

- **当** StatefulSet 命中自动动作路径
- **那么** 系统必须记录 `shadowAction` 并输出 `manual-intervention-required` 语义

### 需求:StatefulSet 关联键一致性

系统必须保证 StatefulSet 在审计、事件、指标中的关联键一致，支持单请求全链路追踪。

#### 场景: 全链路检索

- **当** 运维人员按 correlation key 查询
- **那么** 必须能够在 audit、event、metric 中检索到同一 StatefulSet 请求

## MODIFIED Requirements

## REMOVED Requirements
