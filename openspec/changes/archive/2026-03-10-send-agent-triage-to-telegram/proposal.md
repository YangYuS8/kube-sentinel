## 为什么

Agent v1 的分诊输出契约已经建立，但夜间值班体验还停留在“Agent 能生成解释，运维仍需主动去拉取”的阶段。为了让“我睡觉时哨兵先看一轮，再把可接手的 incident card 发到手机上”真正成立，需要把已有的 Agent v1 分诊结果稳定送达 Telegram，并把通知边界收在一个足够小、足够实用的闭环里。

## 变更内容

- 将已有 Agent v1 的五段式分诊结果映射为 Telegram 短版 ping 和长版 incident card。
- 跑通 `auto-tried`、`blocked`、`recovered` 三类 incident 的 Telegram 通知闭环。
- 为 Telegram 消息补齐最小继续排查入口，包括 `HealingRequest`、Grafana 提示和 `kubectl` 提示。
- 将该能力限定为“发送已有分诊结果”，而不是重新设计 Agent 输出、增加新分析模型或扩展多通道路由。

## 功能 (Capabilities)

### 新增功能
- `telegram-triage-delivery`: 定义 Agent v1 分诊结果发送到 Telegram 的通知契约、模板与闭环行为。

### 修改功能
- `ops-console-integration`: 收紧 Telegram 在多入口体验中的角色，明确其是主动到达层，而不是新的控制台或路由中心。
- `observability`: 明确 Telegram 消息所需的最小关联信息与追查入口。

## 影响

- 受影响的代码主要集中在 Agent 输出消费层、通知发送入口和少量 observability/文档表面。
- 受影响的产品表面包括 Telegram 短版 ping、长版 incident card，以及 incident 的继续排查入口提示。
- 该变更不应扩大到多通道路由、复杂升级打扰、根因增强或新的自动修复动作。
