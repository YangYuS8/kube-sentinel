## 1. 引入 oncall state 翻译层

- [x] 1.1 定义 runtime phase 到 oncall state 的稳定映射函数与最小契约
- [x] 1.2 明确 `observing`、`blocked`、`auto-tried`、`recovered` 四类值班状态的标题和语义说明
- [x] 1.3 保证 phase 仍保留为事实层，但默认值班表面优先使用 oncall state

## 2. 对齐 Agent 与 Telegram

- [x] 2.1 调整 Agent 输出，使 `what happened` 表达真实 phase，`what runtime did` 优先使用 oncall state 翻译
- [x] 2.2 调整 Telegram 标题和首句，使其基于 oncall state 而不是 phase 原值
- [x] 2.3 为 `observing`、`blocked`、`auto-tried`、`recovered` 四类状态补齐样例或测试验证

## 3. 对齐闭环演练与文档

- [x] 3.1 更新 drill/验收脚本，使其同时验证真实 phase 和 oncall state
- [x] 3.2 更新 README 与值班相关文档，解释为什么需要 oncall state 翻译层
- [x] 3.3 在本地或测试环境中补一轮值班闭环验收，确认 phase 与 oncall state 不再互相冲突
