## 1. Telegram 发送链路

- [x] 1.1 定义 Telegram 最小配置与发送入口，只支持 Bot Token 和目标 Chat 标识所需的最小闭环
- [x] 1.2 将现有 Agent v1 结构化输出映射为 Telegram 短版 ping 和长版 incident card
- [x] 1.3 确保 Telegram 发送失败不会阻塞 runtime 主流程，并保留最小可观察结果

## 2. 三类 incident 通知闭环

- [x] 2.1 跑通 `auto-tried` 通知，从 Agent 输出到 Telegram 实际发送
- [x] 2.2 跑通 `blocked` 通知，从 Agent 输出到 Telegram 实际发送
- [x] 2.3 跑通 `recovered` 通知，从 Agent 输出到 Telegram 实际发送

## 3. 继续排查入口与验证

- [x] 3.1 为 Telegram 长版 incident card 补齐 `HealingRequest`、Grafana 和 `kubectl` 继续排查提示
- [x] 3.2 为 Telegram 发送成功/失败、三类通知模板和消息映射补齐测试或样例验证
- [x] 3.3 更新 README 与运维文档，明确 Telegram 是 V1 唯一主动通知通道
