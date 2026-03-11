## 新增需求

### 需求:Telegram 标题必须基于 oncall state 生成
系统必须让 Telegram 通知标题和首句优先基于 oncall state 生成，禁止直接以内部 phase 原值作为夜间第一视图。

#### 场景: PendingVerify 进入 Telegram 通知
- **当** 某个 incident 的真实 phase 为 `PendingVerify`
- **那么** Telegram 标题必须表达为 `observing` 语义，而不得直接显示 `PendingVerify`

## 修改需求

### 需求:Telegram 发送必须保留短版和长版两层消息
系统必须为 Telegram 发送保留短版 ping 和长版 incident card 两层消息结构，且两层消息都必须基于 oncall state 而不是 phase 原值组织标题和首句。

#### 场景: 发送 blocked incident
- **当** 某个 incident 的 oncall state 为 `blocked`
- **那么** 系统必须以 `blocked` 语义生成 Telegram 短版和长版消息，而不是直接暴露内部 phase 文本

## 移除需求
