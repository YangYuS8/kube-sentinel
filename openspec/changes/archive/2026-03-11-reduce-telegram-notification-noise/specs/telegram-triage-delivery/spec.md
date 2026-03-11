## 新增需求

### 需求:Telegram 发送必须按 oncall state 决定默认强度
系统必须根据 oncall state 决定 Telegram 通知的默认强度，并明确 `observing` 不得与 `blocked` 采用同等重复发送策略。

#### 场景: observing 和 blocked 的发送策略不同
- **当** 两个 incident 分别处于 `observing` 和 `blocked`
- **那么** 系统必须允许 `blocked` 采用更强的提醒策略，而对 `observing` 采用更克制的发送策略

## 修改需求

### 需求:Telegram 发送必须保留短版和长版两层消息
系统必须为 Telegram 发送保留短版 ping 和长版 incident card 两层消息结构，且发送策略必须考虑去重和抑制规则，禁止在同一 oncall state 下重复发送等价的短版/长版消息。

#### 场景: 相同 oncall state 重复触发发送
- **当** 同一 incident 在同一 oncall state 下再次进入 Telegram 发送路径
- **那么** 系统必须抑制等价的重复消息

## 移除需求
