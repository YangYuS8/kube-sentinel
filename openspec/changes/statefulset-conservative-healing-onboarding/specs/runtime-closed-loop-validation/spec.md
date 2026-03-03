## ADDED Requirements

### 需求:StatefulSet 只读闭环验收

系统必须提供 StatefulSet 只读闭环验收用例，覆盖接入、门禁、阻断、影子执行与人工介入建议输出。

#### 场景: StatefulSet 闭环演练

- **当** 执行闭环演练脚本
- **那么** 系统必须输出 StatefulSet 只读路径的通过/失败结果与证据摘要

## MODIFIED Requirements

### 需求:运行时闭环验收基线

系统必须提供可重复执行的闭环验收流程，并将 StatefulSet 只读路径纳入强断言，验证“无写操作副作用”。

#### 场景: StatefulSet 无写操作断言

- **当** 演练目标为 StatefulSet
- **那么** 系统必须断言无 L1/L2 写操作且存在只读阻断证据

### 需求:误动作与误阻断指标门槛

系统必须在验收报告中分别输出 Deployment 与 StatefulSet 的误放行率、误阻断率与重复执行率。

#### 场景: 分工作负载门槛校验

- **当** 验收指标统计完成
- **那么** 系统必须按 workloadKind 维度校验发布门槛并在超阈值时阻断发布

## REMOVED Requirements
