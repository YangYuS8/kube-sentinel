## 上下文

当前系统已经形成 runtime phase、Agent 五段式输出和 Telegram incident card 三层表面，但真实本地演练表明：实现状态机和夜间值班心智并不一致。像 `PendingVerify` 这样的内部阶段会直接泄露到 Telegram 文案、Agent 解释和 drill 断言中，导致值班人需要理解机器状态才能知道“系统正在观察、已经阻断、已经自动尝试还是已经恢复”。

这个变更的目标不是重写状态机，而是在 runtime phase 之上增加一层稳定的 oncall state 翻译，使系统内部语言和夜间值班语言解耦。

## 目标 / 非目标

**目标：**
- 定义从 runtime phase 到 oncall state 的稳定映射。
- 让 Telegram 标题和首句优先使用 oncall state，而不是 phase 原值。
- 让 Agent 输出区分事实层（phase）和值班层（oncall state）。
- 让 drill/验收脚本同时保留系统断言和 oncall 断言。

**非目标：**
- 不重写现有 runtime 状态机。
- 不新增自动修复动作。
- 不扩展新的通知通道。
- 不在本变更中做大规模 legacy 清理。

## 决策

### 决策 1：引入独立的 oncall state 翻译层

系统将 runtime phase 映射为值班语义状态，而不是要求夜班用户直接理解 phase。

建议映射：
- `Pending` / `PendingVerify` -> `observing`
- `Blocked` / `L3` -> `blocked`
- `L1` / `Completed` -> `auto-tried`
- `Suppressed` -> `recovered`

原因：phase 是状态机语言，oncall state 才是产品语言。二者职责不同，不应继续混用。

考虑过的替代方案：
- 直接在 Telegram 和 Agent 中继续暴露 phase：被拒绝，因为这会持续泄露实现细节。
- 隐藏 `PendingVerify`，强行压回三类状态：被拒绝，因为真实行为已经证明它是重要状态。

### 决策 2：Telegram 基于 oncall state 生成标题

Telegram 标题和首句必须优先使用 oncall state，例如：
- `observing` -> `[WAIT] Sentinel 正在观察`
- `blocked` -> `[WARN] Sentinel 已阻断自动处理`
- `auto-tried` -> `[INFO] Sentinel 已自动尝试恢复`
- `recovered` -> `[OK] Sentinel 判定暂不需要动作`

原因：标题层需要最短路径表达“现在我要不要接手”，而不是表达完整实现状态。

考虑过的替代方案：
- 在 Telegram 标题中直接显示 phase：被拒绝，因为 `PendingVerify`、`Suppressed` 这类术语不适合夜班第一视图。

### 决策 3：Agent 同时保留事实层和值班层

Agent 的 `what happened` 可保留 phase 作为事实字段，但 `what runtime did` 和整体值班语气必须优先基于 oncall state 翻译。

原因：这样既保留调试真实性，又不会让用户直接暴露在内部状态机术语里。

考虑过的替代方案：
- 完全移除 phase：被拒绝，因为它对调试和细节追踪仍有价值。

### 决策 4：drill 改为双层验收

drill/验收脚本必须同时验证：
- 系统断言：phase、gate、action、block reason
- 值班断言：oncall state、Telegram 类别、Agent 解释心智

原因：仅验证 phase 已不能代表夜班体验是否正确。

考虑过的替代方案：
- 继续只断言 phase：被拒绝，因为这正是当前演练漂移的根源。

## 风险 / 权衡

- [风险] 同时存在 phase 和 oncall state 会让一些输出看起来重复 → 缓解措施：默认对外先显示 oncall state，phase 只出现在详情层。
- [风险] `observing` 是否总要通知仍可能存在争议 → 缓解措施：本变更只先定义语义层，不强绑定最终通知强度策略。
- [风险] 旧 drill 和旧文案仍可能残留旧心智 → 缓解措施：本变更将这类对齐纳入明确任务。

## Migration Plan

1. 先引入 oncall state 映射函数和最小契约。
2. 对齐 Telegram 标题、Agent 解释层与 drill 断言。
3. 再根据新语义评估通知去噪与后续 UX 优化。

## Open Questions

- `observing` 默认是否总要主动通知，还是应作为后续通知去噪变更的一部分处理？
- 是否需要把 oncall state 持久化到 `HealingRequest.status`，还是保持为派生语义层？
