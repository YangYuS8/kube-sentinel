## 为什么

Telegram 通知闭环已经打通，但真实本地演练暴露出一个更急迫的问题：在状态反复 patch、发送失败或运行时仍在观察的情况下，Telegram 很容易变成新的噪音源。对于一个夜间值班工具来说，消息能发出来还不够，更关键的是它不能反复刷屏、不能把中间态过度升级成告警、也不能在发送失败时持续制造噪音。

## 变更内容

- 为 Telegram 通知增加最小去重和降噪策略，避免同一 incident 在短时间内重复刷屏。
- 明确 `observing`、`blocked`、`auto-tried`、`recovered` 四类 oncall state 的默认通知强度和发送规则。
- 为发送失败增加降噪保护，避免在 Telegram 不可用时重复打出失败事件。
- 保持该变更聚焦于“减少 Telegram 通知噪音”，不扩展到多通道路由、根因增强或新的自动修复动作。

## 功能 (Capabilities)

### 新增功能
- `telegram-notification-noise-control`: 定义 Telegram 通知的去重、降噪和失败抑制策略。

### 修改功能
- `telegram-triage-delivery`: 收紧 Telegram 消息的发送时机和重复发送边界。
- `oncall-state-translation`: 明确不同 oncall state 的默认通知强度。
- `observability`: 为通知去重、抑制和发送失败提供最小可观察语义。

## 影响

- 受影响的代码主要集中在 Telegram 发送触发逻辑、通知状态记录、失败事件记录和少量 Agent/状态映射辅助逻辑。
- 受影响的产品表面包括夜间 Telegram 消息数量、`observing` 的默认打扰程度以及发送失败时的可见行为。
- 该变更不涉及新的通知通道、不涉及重新设计 Agent 分诊模型，也不做大规模 legacy 清理。
