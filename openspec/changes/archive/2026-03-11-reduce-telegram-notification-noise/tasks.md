## 1. 引入 Telegram 去重与抑制基础

- [x] 1.1 定义 Telegram 通知去重键，至少覆盖 incident 身份与 oncall state
- [x] 1.2 为 Telegram 发送结果增加已发送/已抑制/已失败三类最小记录
- [x] 1.3 确保同一 incident 在同一 oncall state 下不会重复发送等价消息

## 2. 收敛不同 oncall state 的默认发送策略

- [x] 2.1 为 `observing` 定义低噪音发送策略，避免观察态样本推进持续刷屏
- [x] 2.2 保持 `blocked`、`auto-tried`、`recovered` 的通知强度清晰，并允许状态切换时重新发送
- [x] 2.3 为 Telegram 发送失败增加重复失败抑制，避免不可用时持续刷失败事件

## 3. 验证与文档

- [x] 3.1 为通知去重、抑制和失败降噪补齐测试
- [x] 3.2 在本地演练中验证同一 incident 不再重复刷 Telegram
- [x] 3.3 更新 README/值班文档，说明 Telegram 的默认降噪策略
