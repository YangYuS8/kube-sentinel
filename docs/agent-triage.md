# Agent v1 Triage

Kube-Sentinel 的 Agent v1 是一个夜间值班分诊器，而不是自治执行器。

Agent v1 只负责：

- 解释发生了什么
- 说明 runtime 做了什么
- 收敛当前焦点分类
- 给出下一步建议
- 生成可复制的交接说明

## 五段式输出

Agent v1 为单个 incident 固定输出以下五段：

- `what happened`
- `what runtime did`
- `current focus`
- `next steps`
- `handoff`

当前 `current focus` 采用有限分类：

- `startup-failure`
- `config-or-dependency`
- `safety-blocked`
- `transient-or-recovered`
- `manual-follow-up`
- `insufficient-evidence`

## 输入分层

Agent v1 默认只消费三层输入：

- `core`: workload 身份、phase、lastAction、blockReasonCode、lastError、nextRecommendation、recommendationType、correlationKey
- `evidence`: snapshot 状态、circuit breaker、最近 runtime event、最小趋势指标
- `legacy`: Deployment L2/L3、StatefulSet 自动化、旧发布治理语义

默认情况下，legacy 字段不应进入 Agent v1 主解释路径。

## Telegram 通知

V1 只支持 Telegram 主动通知，并将单个 incident 映射为两层消息：

- 短版 ping：用于快速判断是否需要查看
- 长版 incident card：用于接手和继续排查

支持的通知类别：

- `auto-tried`
- `blocked`
- `recovered`

当前主动通知通道只支持 Telegram。最小配置：

- `KUBE_SENTINEL_TELEGRAM_BOT_TOKEN`
- `KUBE_SENTINEL_TELEGRAM_CHAT_ID`
- `KUBE_SENTINEL_TELEGRAM_BASE_URL`（可选，测试时可覆盖）

如果 Telegram 发送失败，runtime 主流程仍继续，失败结果会通过事件或可观察记录暴露。

## 继续排查入口

Telegram incident card 必须保留以下跳转语义：

- 对象入口：`HealingRequest`
- 趋势入口：Grafana namespace/workload 面板
- 精查入口：`kubectl describe` 等最小命令提示
