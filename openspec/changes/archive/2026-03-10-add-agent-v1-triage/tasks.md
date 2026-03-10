## 1. 定义 Agent v1 输入与输出契约

- [x] 1.1 梳理现有 `HealingRequest.status` 字段，明确哪些属于 Agent v1 的 core、evidence、legacy 三层输入
- [x] 1.2 为 Agent v1 组装固定五段式输出：`what happened`、`what runtime did`、`current focus`、`next steps`、`handoff`
- [x] 1.3 为 `current focus` 定义有限分类集合及 `insufficient-evidence` 回退路径

## 2. 收敛状态语义与观察证据

- [x] 2.1 调整状态语义或派生逻辑，使 Agent 能稳定读取建议类别、阻断原因、动作结果和关联键
- [x] 2.2 收敛 Agent 默认依赖的运行时证据，明确哪些审计、事件和趋势字段进入 V1 解释路径
- [x] 2.3 为 legacy 状态语义建立隔离或降噪策略，避免 Deployment L2/L3、StatefulSet 和旧治理逻辑污染 Agent v1 默认输入面

## 3. 交付 Telegram 通知表面

- [x] 3.1 定义 Telegram 短版 ping 和长版 incident card 的结构与字段映射
- [x] 3.2 将 `auto-tried`、`blocked`、`recovered` 三类 incident 映射到对应通知模板
- [x] 3.3 为 Telegram 通知补齐关联入口提示，使值班人员可继续进入 Agent、Headlamp、Grafana 和 kubectl 路径

## 4. 验证与文档

- [x] 4.1 为 Agent v1 输入分层、焦点分类和五段式输出补齐测试或契约验证
- [x] 4.2 为 Telegram 通知模板和三类 incident 场景补齐测试或样例验证
- [x] 4.3 更新 README、运维文档和相关说明，明确 Agent v1 是值班分诊器而不是自治执行器
