## 上下文

当前仓库已经完成了“轻量哨兵 + Agent 副驾驶”的世界观收缩，但 Agent 本身仍然更像一个方向，而不是一个正式定义的产品表面。现有 `HealingRequest.status` 已经包含 `incidentSummary`、`recommendationType`、`handoffNote` 等基础字段，runtime 也能输出 `phase`、`lastAction`、`blockReasonCode`、`nextRecommendation` 等核心语义，但这些字段还没有被组织成一个稳定的 Agent v1 输入/输出契约。

另外，历史代码仍然保留了 Deployment L2/L3、StatefulSet 自动化和部分旧治理语义。如果直接让 Agent 读取所有状态与审计内容，很容易重新滑回“复杂平台解释层”，而不是一个面向夜间值班的分诊器。因此，本变更的关键不是增强自动执行，而是定义一个清晰、克制、可通知的 Agent v1 表面。

## 目标 / 非目标

**目标：**
- 为 Agent v1 定义固定输出契约：`what happened`、`what runtime did`、`current focus`、`next steps`、`handoff`。
- 为 Agent v1 定义三层输入模型：`core`、`evidence`、`legacy`。
- 为 Telegram 定义 V1 唯一通知表面，包括短版 ping 和长版 incident card。
- 明确 Agent v1 是“分诊器/解释器”，负责缩小排查范围和提供建议，而不是做开放式根因判定或生产写动作。
- 为后续减重提供依据：哪些 runtime 状态与 observability 语义是 Agent v1 默认输入，哪些应退到 legacy。

**非目标：**
- 不在本变更中引入新的自动修复动作。
- 不在本变更中实现多通道路由或复杂通知升级策略；V1 只聚焦 Telegram。
- 不在本变更中重构整个 orchestrator 或全面删除历史 L2/L3/StatefulSet 逻辑。
- 不在本变更中承诺 Agent 给出确定性根因、自动命令生成或自由执行。

## 决策

### 决策 1：Agent v1 采用固定五段式输出

Agent v1 输出固定为五段：`what happened`、`what runtime did`、`current focus`、`next steps`、`handoff`。

原因：固定结构比自由文本更容易被运维快速扫描，也更容易稳定映射到 Telegram 通知卡片和后续控制台/CLI 消费面。

考虑过的替代方案：
- 直接输出自由文本总结：被拒绝，因为结构不稳定、难以复用到通知和交接。
- 只保留 `incidentSummary` 和 `handoffNote` 两个字段：被拒绝，因为它们不足以表达“当前焦点”和“下一步建议”的结构化边界。

### 决策 2：Agent v1 输入采用 core / evidence / legacy 三层分流

Agent v1 默认只消费最小 `core` 字段；`evidence` 作为增强判断的补充；`legacy` 默认不进入解释主路径。

原因：当前状态字段和观测语义已经混入了历史 L2/L3、StatefulSet 和治理逻辑，如果没有输入分层，Agent 会重新长成一个复杂平台解释器。

分层原则：
- `core`: 直接驱动输出所需的最小 incident 字段
- `evidence`: 用于增强 `current focus` 和 `next steps` 的运行时证据
- `legacy`: 历史自动化或治理语义，默认不进入 Agent v1 主输入

考虑过的替代方案：
- 让 Agent 默认读取所有 `HealingRequest.status` 字段：被拒绝，因为会放大 legacy 语义。
- 只读 `incidentSummary` 与 `handoffNote`：被拒绝，因为派生字段不足以支撑稳定分诊。

### 决策 3：Agent v1 是 triage engine，而不是 root-cause engine

Agent v1 的 `current focus` 只允许输出有限的焦点分类，例如 `startup-failure`、`config-or-dependency`、`safety-blocked`、`transient-or-recovered`、`manual-follow-up`、`insufficient-evidence`。

原因：V1 的目标是减少夜间认知负担，而不是在证据不足时冒充根因分析器。有限分类更适合夜间值班，也更容易保持谨慎表达。

考虑过的替代方案：
- 直接要求 Agent 输出根因：被拒绝，因为对现有输入质量和覆盖面来说不可靠。
- 完全不做焦点分类，只输出事实：被拒绝，因为无法体现 Agent 的真实价值。

### 决策 4：Telegram 作为 V1 唯一通知通道

V1 只支持 Telegram，并定义两层通知：短版 ping 和长版 incident card。

原因：Telegram 足够轻量、适合个人值班、支持通知与详情共存，也能避免在 V1 过早扩展为复杂通知系统。

考虑过的替代方案：
- 同时支持多个通道：被拒绝，因为会分散 V1 精力。
- 只做对象状态，不做主动通知：被拒绝，因为夜间值班价值会明显下降。

### 决策 5：减重以“保护 Agent 输入面”为目标，而不是先做大扫除

本变更只要求明确哪些字段/语义属于 Agent v1 的 `core`、`evidence` 和 `legacy`，并以此指导后续代码减重；不在本次变更中承诺完成大规模物理删除。

原因：先有输入边界，后有减重标准。否则“清理旧代码”会重新变成无边界工程。

考虑过的替代方案：
- 先发起大规模 legacy 清理：被拒绝，因为会让产品方向和代码整理交织在一起。

## 风险 / 权衡

- [风险] 五段式输出可能显得过于模板化 → 缓解措施：允许短版/长版通知复用同一契约，但不要求每个入口展示全部字段。
- [风险] core/evidence/legacy 分层会暴露当前状态字段过胖的问题 → 缓解措施：本变更先固化分层标准，后续再按标准做减重。
- [风险] Telegram-only 看起来像临时方案 → 缓解措施：在设计中明确它是 V1 的 intentional scope，而不是永远唯一通道。
- [风险] 焦点分类过少可能让建议显得保守 → 缓解措施：允许 `current focus=insufficient-evidence`，并优先保证可信度而非激进判断。

## Migration Plan

1. 先在规格层定义 Agent v1 输出契约、输入分层与 Telegram 表面。
2. 基于新规格梳理现有 `HealingRequest.status` 字段，标注哪些属于 Agent v1 默认输入。
3. 在实现阶段优先做最小输出组装和 Telegram 通知，再逐步压低 legacy 字段在默认解释路径中的存在感。
4. 若后续决定扩展多通道或更深分析能力，应通过新的独立变更引入，而不是扩张本变更范围。

## Open Questions

- `current focus` 的枚举是否应在 V1 就完全固定，还是允许保留一个有限扩展槽位？
- Telegram 长版 incident card 应直接由 Agent 生成，还是由 runtime/通知层基于结构化输出拼装？
- 是否需要在 `HealingRequest.status` 中显式标记每个字段的 `input tier`，还是先只在设计和实现中约定？
