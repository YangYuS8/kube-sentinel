## 上下文

`add-agent-v1-triage` 已经定义并实现了 Agent v1 的输入分层、五段式输出和 Telegram 通知模板，但当前 Telegram 仍然更像一个静态表面：模板存在，真正的发送链路、配置边界和交付闭环还没有成为一个独立、可审查的产品能力。

这个变更的目标不是重新设计 Agent，也不是扩展新的分析能力，而是把已有 Agent v1 分诊结果稳定、安全、克制地送到 Telegram，让夜间值班体验真正形成闭环。

## 目标 / 非目标

**目标：**
- 为 Agent v1 增加实际的 Telegram 发送闭环。
- 只支持已有三类通知：`auto-tried`、`blocked`、`recovered`。
- 保持短版 ping + 长版 incident card 两层消息结构。
- 让 Telegram 消息携带最小继续排查入口：对象、趋势、精查提示。
- 保持这个变更是“发送已有分诊结果”，而不是“扩展新的 Agent 能力”。

**非目标：**
- 不增加多通道路由或通知升级策略。
- 不重新定义 Agent v1 的五段式输出或焦点分类。
- 不引入新的自动修复动作。
- 不在本变更中做 legacy 代码的大规模清理。

## 决策

### 决策 1：Telegram 发送层必须消费已有 Agent 输出，而不是重复实现分诊逻辑

Telegram 发送层只负责把已有 Agent v1 输出映射成短版 ping 和长版 incident card，禁止在发送路径中重新实现 incident 分诊或根因判断。

原因：如果通知层再次做解释，就会出现“双份逻辑”，并让产品边界重新变模糊。

考虑过的替代方案：
- 在发送层直接读取 `HealingRequest` 拼 Telegram 文案：被拒绝，因为会绕开 Agent v1 契约。

### 决策 2：V1 只支持 Telegram 单通道配置

发送闭环只支持 Telegram Bot API 所需的最小配置，不在本变更中抽象成通用通知提供方。

原因：V1 需要的是可用，而不是通知平台。过早抽象会让范围重新膨胀。

考虑过的替代方案：
- 先做 provider 接口支持多通道：被拒绝，因为会把范围拉向通知中台。

### 决策 3：发送策略保持和 Agent 模板一一对应

`auto-tried`、`blocked`、`recovered` 三类 incident 直接映射到已有 Telegram 模板，不新增第四类或额外升级链。

原因：这三类已经足够覆盖 V1 核心值班心智；增加更多状态只会带来新的通知设计负担。

考虑过的替代方案：
- 在本变更里加入 escalation、mute、batching：被拒绝，因为超出 V1 范围。

## 风险 / 权衡

- [风险] Telegram Bot API 发送失败会让通知闭环中断 → 缓解措施：发送失败必须可记录、可观察，并不得影响 runtime 主流程。
- [风险] 发送配置过于简陋可能让后续扩展困难 → 缓解措施：保持配置最小，但为未来独立变更保留扩展空间。
- [风险] Telegram 文案与 Agent 输出发生漂移 → 缓解措施：发送层只消费 Agent 结构化输出，不自行重组另一套语义。

## Migration Plan

1. 先为 Telegram 发送定义最小配置和发送入口。
2. 将现有 Agent v1 的结构化输出直接映射到 Telegram Bot API 请求。
3. 通过一两个典型 incident 场景验证消息已真正发出，并确保失败不会阻塞 runtime。
4. 后续若要支持更多渠道，应通过新的独立变更引入。

## Open Questions

- Telegram 配置是否只需要 `bot token + chat id`，还是需要为个人/群组场景额外预留最小开关？
- 发送失败时，最小可观察策略应该是 runtime event、audit 记录，还是两者都要？
