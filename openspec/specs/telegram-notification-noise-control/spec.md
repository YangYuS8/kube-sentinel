## 目的

定义 Kube-Sentinel 如何降低 Telegram 通知噪音，避免同一 incident 在相同值班语义下反复刷屏，并在 Telegram 不可用时抑制重复失败事件。

## 需求

### 需求:Telegram 通知必须具备最小去重能力
系统必须对同一 incident 的 Telegram 通知进行最小去重，至少基于 incident 身份和 oncall state 避免重复发送，禁止同一语义状态在短时间内反复刷屏。

#### 场景: 同一 blocked incident 多次状态 patch
- **当** 同一个 incident 在 `blocked` 语义下发生多次无本质变化的状态更新
- **那么** 系统必须抑制重复 Telegram 消息

### 需求:observing 必须采用低噪音策略
系统必须对 `observing` 语义采用低噪音策略，禁止因样本推进或重复 reconcile 在观察窗口内持续发送 Telegram 提醒。

#### 场景: observing 样本数递增
- **当** 某个 incident 仍处于 `observing` 状态，但样本数或中间证据发生推进
- **那么** 系统不得因此持续发送新的 Telegram 提醒

### 需求:Telegram 发送失败必须被抑制
系统必须对同一 incident 的 Telegram 发送失败进行最小抑制，禁止在 Telegram 不可用时持续输出重复失败事件。

#### 场景: Telegram 服务暂时不可用
- **当** 同一 incident 在短时间内多次触发 Telegram 发送失败
- **那么** 系统必须限制重复失败记录的数量，并保留最小可观察结果
