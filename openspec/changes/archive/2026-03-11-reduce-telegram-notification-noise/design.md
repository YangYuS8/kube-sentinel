## 上下文

Telegram 通知闭环已经打通，oncall state 翻译层也已经建立，但真实本地演练暴露出另一个更贴近值班体验的问题：在状态频繁 patch、观察态持续推进或发送失败时，同一个 incident 可能反复触发 Telegram 发送。对于夜班工具来说，这会迅速把“解释性通知”变成新的噪音源。

当前系统缺少一个面向 Telegram 的最小通知去重/抑制模型，因此这次设计需要在不引入通知平台复杂度的前提下，给 Telegram 增加稳定的降噪边界。

## 目标 / 非目标

**目标：**
- 为 Telegram 通知定义最小去重和抑制规则。
- 明确不同 oncall state 的默认通知强度和发送时机。
- 避免发送失败时持续刷 `TelegramNotificationFailed`。
- 保持这次变更聚焦于 Telegram 降噪，而不是扩展通知平台能力。

**非目标：**
- 不支持多通道路由。
- 不重新设计 Agent 分诊模型。
- 不引入新的自动修复动作。
- 不在本变更中做全局 observability 重构。

## 决策

### 决策 1：通知去重必须基于 incident 身份与 oncall state

系统将以 `correlationKey`、对象身份和 `oncall state` 作为 Telegram 发送去重的最小键，避免同一 incident 在同一语义状态下重复发送。

原因：夜班人真正关心的是“这个 incident 现在处于什么值班语义”，不是每次 status patch 的细节。

考虑过的替代方案：
- 仅按对象名去重：被拒绝，因为不同 oncall state 切换需要重新通知。
- 按完整 status 哈希去重：被拒绝，因为会过于敏感，仍然容易刷屏。

### 决策 2：`observing` 默认弱化或抑制

`observing` 不应像 `blocked` 一样默认高强度通知。V1 中它应默认弱化，或者仅在从非 observing 切换到 observing 时发送一次。

原因：`observing` 是中间态，若每次样本推进都发消息，会快速制造噪音。

考虑过的替代方案：
- 对 observing 每次都发：被拒绝，因为演练已经证明这会产生重复通知风险。

### 决策 3：发送失败需要抑制，而不是逐次重复记录

对于同一 incident 的 Telegram 发送失败，应在一定窗口内抑制重复失败事件，避免 Telegram 不可用时持续刷屏。

原因：值班人不需要看到几十条完全相同的 `TelegramNotificationFailed`。

考虑过的替代方案：
- 完全静默失败：被拒绝，因为会失去可观察性。

## 风险 / 权衡

- [风险] 去重过强可能漏掉有价值状态切换 → 缓解措施：去重键必须包含 oncall state，而不是只看对象名。
- [风险] `observing` 弱化后可能让人错过某些重要中间态 → 缓解措施：先保留一次进入 observing 的通知能力，但抑制重复发送。
- [风险] 失败抑制会减少可观察数据 → 缓解措施：保留最小失败记录，但避免重复爆炸。

## Migration Plan

1. 先引入 Telegram 发送去重键和最小抑制存储。
2. 对齐 `observing`、`blocked`、`auto-tried`、`recovered` 的默认发送策略。
3. 在本地演练中验证同一 incident 不再反复刷屏。

## Open Questions

- 去重窗口应完全时间驱动，还是允许部分状态切换立即打破窗口？
- `recovered` 是否也需要弱化，还是应始终保留一次收尾通知？
