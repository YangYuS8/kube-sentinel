## 新增需求

### 需求:Telegram 必须作为 V1 唯一主动通知通道
系统必须将 Telegram 定义为 Agent v1 的唯一主动通知通道，并禁止在 V1 范围内同时要求多通道路由或复杂通知编排能力。

#### 场景: 单个 incident 进入主动通知路径
- **当** Agent 需要为某个 incident 主动通知值班人员
- **那么** 系统必须通过 Telegram 发送通知，而不得要求其他通知通道作为 V1 前置条件

### 需求:Agent 通知必须区分短版和长版
系统必须为 Telegram 通知定义短版 ping 和长版 incident card 两层结构，禁止让夜间通知在第一条消息中同时承担唤醒和完整事故说明两种职责。

#### 场景: 发送 blocked incident 通知
- **当** 某个 incident 需要主动通知值班人员且当前状态为 blocked 或 manual follow-up
- **那么** 系统必须先发送可快速扫描的短版通知，并提供结构完整的长版 incident card

## 修改需求

### 需求:V1 运维体验必须采用多入口协同模型
系统必须将 V1 运维体验定义为 Agent、Telegram、Headlamp、Grafana 和 kubectl 的协同模式；Telegram 负责主动到达，Agent 负责解释，Headlamp/Grafana/kubectl 负责对象、趋势和精确接管，禁止将独立厚控制台视为首发前置条件。

#### 场景: 运维接收并处理单个 incident
- **当** 运维收到某个夜间 incident 的主动通知
- **那么** 系统必须允许其先在 Telegram 中查看 incident card，再通过 Agent、Headlamp、Grafana 或 kubectl 继续接手和排查

## 移除需求
