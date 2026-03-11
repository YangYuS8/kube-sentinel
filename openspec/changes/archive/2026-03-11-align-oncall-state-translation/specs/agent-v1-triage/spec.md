## 新增需求

### 需求:Agent 必须区分事实层和值班层
系统必须让 Agent 输出同时支持事实层和值班层：`what happened` 可以表达真实 phase，`what runtime did` 和值班解释必须优先基于 oncall state，而不是直接把 phase 原样暴露为值班结论。

#### 场景: phase 为 PendingVerify 时生成值班解释
- **当** Agent 读取到 incident 的真实 phase 为 `PendingVerify`
- **那么** Agent 必须将其解释为“正在观察”，而不是把 `PendingVerify` 直接当作值班结论输出

## 修改需求

### 需求:Agent v1 必须输出固定五段式结构
系统必须为 Agent v1 提供固定输出契约，至少包括 `what happened`、`what runtime did`、`current focus`、`next steps` 和 `handoff` 五段；其中 `what happened` 可保留 phase 作为事实字段，但 `what runtime did` 必须优先使用 oncall state 解释当前值班语义。

#### 场景: 生成五段式输出
- **当** Agent 为某个 incident 组装五段式输出
- **那么** 输出必须同时保留真实事实和可理解的值班语义，而不得只暴露内部状态机术语

## 移除需求
