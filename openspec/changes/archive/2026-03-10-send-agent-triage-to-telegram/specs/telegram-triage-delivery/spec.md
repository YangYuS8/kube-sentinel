## 新增需求

### 需求:系统必须将 Agent v1 分诊结果发送到 Telegram
系统必须能够将已有 Agent v1 的结构化分诊结果发送到 Telegram，禁止要求值班人员必须主动查询对象或控制台才能获得第一份 incident 解释。

#### 场景: 单个 incident 进入主动通知路径
- **当** 某个 incident 满足主动通知条件
- **那么** 系统必须将对应的 Agent v1 分诊结果发送到 Telegram

### 需求:Telegram 发送必须保留短版和长版两层消息
系统必须为 Telegram 发送保留短版 ping 和长版 incident card 两层消息结构，禁止把第一条消息设计成同时承担唤醒与完整事故报告的单条长文。

#### 场景: 发送 blocked incident
- **当** 某个 blocked incident 进入 Telegram 通知路径
- **那么** 系统必须先发送短版 ping，再提供长版 incident card

### 需求:Telegram 发送不得绕过 Agent 输出契约
系统必须让 Telegram 发送层直接消费已有 Agent v1 输出，禁止在发送路径中重新实现焦点分类、根因判断或另一套通知语义。

#### 场景: 构建 Telegram 文案
- **当** 系统为 Telegram 构建某条 incident 消息
- **那么** 消息内容必须来源于已有 Agent v1 输出，而不得单独再做分诊逻辑

### 需求:Telegram 发送失败不得阻塞 runtime 主路径
系统必须保证 Telegram 发送失败只影响通知结果，不得阻塞 runtime 主流程、状态收敛或对象更新。

#### 场景: Telegram Bot API 发送失败
- **当** 系统向 Telegram 发送某条 incident 消息失败
- **那么** runtime 主流程必须继续完成，并保留失败可观察信息

## 修改需求

## 移除需求
