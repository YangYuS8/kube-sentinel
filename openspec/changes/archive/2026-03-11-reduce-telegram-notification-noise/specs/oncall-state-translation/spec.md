## 新增需求

### 需求:oncall state 必须支持通知强度策略
系统必须允许基于 oncall state 定义不同的默认通知强度，至少支持 `observing` 的弱化策略和 `blocked` 的强提醒策略。

#### 场景: 基于 oncall state 选择发送策略
- **当** 系统准备为某个 incident 发送 Telegram 消息
- **那么** 系统必须能够基于 oncall state 选择对应的默认强度和去重行为

## 修改需求

## 移除需求
