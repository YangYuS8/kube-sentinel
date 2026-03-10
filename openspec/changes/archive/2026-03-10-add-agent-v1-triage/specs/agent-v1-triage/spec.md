## 新增需求

### 需求:Agent v1 必须输出固定五段式结构
系统必须为 Agent v1 提供固定输出契约，至少包括 `what happened`、`what runtime did`、`current focus`、`next steps` 和 `handoff` 五段，禁止仅返回无法稳定消费的自由文本总结。

#### 场景: 值班人员查询单个 incident
- **当** 值班人员通过 Agent 查询某个 incident 的当前状态
- **那么** Agent 必须按照五段式结构返回解释结果，并保持字段顺序和语义稳定

### 需求:Agent v1 必须采用输入分层模型
系统必须将 Agent v1 输入划分为 `core`、`evidence` 和 `legacy` 三层，禁止让 legacy 自动化或治理语义默认进入 Agent v1 的主解释路径。

#### 场景: Agent 组装单个 incident 输出
- **当** Agent 为某个 incident 组装输出
- **那么** 系统必须优先消费 core 字段，仅在需要时引入 evidence 字段，并默认忽略 legacy 字段

### 需求:Agent v1 必须输出有限焦点分类
系统必须将 Agent v1 的 `current focus` 限定为有限分类集合，至少覆盖 `startup-failure`、`config-or-dependency`、`safety-blocked`、`transient-or-recovered`、`manual-follow-up` 和 `insufficient-evidence`，禁止将 V1 设计成开放式根因生成器。

#### 场景: 证据不足以确定焦点
- **当** incident 当前证据不足以支持收敛的焦点判断
- **那么** Agent 必须输出 `insufficient-evidence` 并给出最小下一步建议，而不得编造确定性根因

### 需求:Agent v1 必须保持只读边界
系统必须将 Agent v1 限定为解释、建议和交接能力，禁止通过 Agent v1 直接引入生产写动作、自由命令执行或自动修复规划。

#### 场景: Agent 发现高风险处理路径
- **当** Agent 判断某个 incident 可能需要高风险、不可逆或超出默认动作集合的处理
- **那么** Agent 必须将其表达为人工建议，而不得转化为直接执行路径

## 修改需求

## 移除需求
