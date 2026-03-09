## 1. 收缩 runtime core

- [x] 1.1 盘点并收缩 `HealingRequest`、controller 与 orchestrator 的核心语义，只保留 Deployment L1、最小安全门禁、快照和人工接管所需状态字段
- [x] 1.2 将 Deployment L2/L3、StatefulSet 自动写路径与其他高风险自动化从默认核心路径中降级为非核心或显式关闭行为
- [x] 1.3 校准 ingestion 与 runtime 分流逻辑，确保 Deployment 保留最小可执行上下文，非 Deployment 默认进入只读路径

## 2. 定义 agent-facing 最小能力

- [x] 2.1 为 incident summary 定义稳定输入字段与输出契约，确保可从 `HealingRequest`、事件和基础指标直接生成摘要
- [x] 2.2 为 next-step recommendation 定义建议类别与边界，明确建议、人工动作和禁止自动执行之间的区分
- [x] 2.3 为 handoff note 定义最小内容模板与关联键要求，确保夜间交接可以直接复用

## 3. 收敛运维入口与观察面

- [x] 3.1 调整对象、指标与文档语义，使 Agent、Headlamp、Grafana 和 kubectl 的分工与跳转关系保持一致
- [x] 3.2 收敛基础 observability 范围，只保留 incident 触发、L1 动作结果、阻断原因、快照结果和最小关联键所需内容
- [x] 3.3 校准最小安装、smoke 与只读退出路径，使值班模式具备快速启用、验证和回退能力

## 4. 更新文档与验证边界

- [x] 4.1 更新 README、安装文档和运维说明，明确 V1 是轻量哨兵而不是自治平台
- [x] 4.2 为新的 runtime core、agent-facing 契约和多入口体验补齐测试或验证用例
- [x] 4.3 复核现有非核心 spec、脚本和文档，明确哪些保留为 experimental，哪些迁移为 operator workflow
