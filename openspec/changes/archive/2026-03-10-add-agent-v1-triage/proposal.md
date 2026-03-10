## 为什么

当前 Kube-Sentinel 已经具备最小 runtime 和基础状态语义，但夜间值班体验仍停留在“对象和事件已经存在，运维仍需自己拼上下文”的阶段。为了真正减少夜间排错时间，系统需要一个清晰收敛的 Agent v1 分诊表面：先解释发生了什么、runtime 做了什么、现在最该看哪里，再通过 Telegram 发出可接手的 incident 卡片。

## 变更内容

- 为 Agent v1 定义最小输出契约，固定为 `what happened`、`what runtime did`、`current focus`、`next steps` 和 `handoff` 五段式结构。
- 为 Agent v1 定义输入分层模型，将现有 runtime 状态与运行时证据分为 `core`、`evidence`、`legacy` 三层，避免旧的 L2/L3、StatefulSet 和发布治理语义污染默认解释路径。
- 引入 Telegram 作为 V1 唯一通知通道，定义 incident card 的短版/长版结构，以及 `auto-tried`、`blocked`、`recovered` 三类夜间通知模式。
- 将 Agent v1 的目标限定为“值班分诊器”，强调缩小排查范围、提供建议和交接，而不是给出开放式根因定论或生产写路径。
- 为后续减重提供明确边界：默认解释层只消费 Agent v1 输入面，旧自治/治理逻辑退为 legacy 或非核心实现。

## 功能 (Capabilities)

### 新增功能
- `agent-v1-triage`: 定义 Agent v1 的输入分层、五段式输出契约与 Telegram incident card。

### 修改功能
- `api-contract-governance`: 收紧 Agent 默认消费的状态字段面，并明确哪些字段属于 core、evidence、legacy。
- `ops-console-integration`: 将 Telegram 纳入 V1 多入口体验，明确 Agent 输出与 Headlamp/Grafana/kubectl 之间的跳转关系。
- `observability`: 为 Telegram incident card 和 Agent 分诊提供最小事件、审计和趋势证据语义。

## 影响

- 受影响的代码主要集中在 `api/v1alpha1`、`internal/healing`、`internal/observability`，以及未来的 Agent/通知集成入口。
- 受影响的产品表面包括 `HealingRequest.status` 的默认解释字段、Telegram 通知内容、Agent 交互结构与多入口文档。
- 受影响的规格包括 Agent 能力定义、状态契约、运维入口和基础观察面；旧的 L2/L3、StatefulSet 自动化和发布治理语义将不再默认进入 Agent v1 输入面。
