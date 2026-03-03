## ADDED Requirements

## MODIFIED Requirements

### 需求:健康 Revision 判定可验证

系统必须基于可验证条件判定历史 Revision 的健康性，并且候选数据来源必须来自真实 Deployment Revision 历史。

#### 场景: 从真实历史选择候选

- **当** 系统执行 L2 回滚候选筛选
- **那么** 系统必须从目标 Deployment 的真实 Revision 历史中选择候选

### 需求:回滚证据可观测输出

系统必须在每次 L2 回滚决策中输出候选筛选证据，并明确记录回滚执行动作与失败恢复结果。

#### 场景: 回滚执行失败

- **当** 回滚动作执行失败并触发恢复
- **那么** 系统必须记录失败原因、恢复动作与最终状态

## REMOVED Requirements
