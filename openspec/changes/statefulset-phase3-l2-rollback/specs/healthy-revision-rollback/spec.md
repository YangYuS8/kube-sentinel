## ADDED Requirements

### 需求:多工作负载健康候选统一语义
系统必须为 Deployment 与 StatefulSet 使用统一的健康候选语义，至少包含稳定窗口、依赖完整性与可回退性证据。

#### 场景: 跨工作负载候选一致
- **当** 编排器对不同工作负载执行 L2 候选筛选
- **那么** 系统必须输出一致结构的候选证据字段

## MODIFIED Requirements

### 需求:健康 Revision 判定可验证
系统必须基于可验证条件判定历史 Revision 的健康性，并支持 Deployment 与 StatefulSet 两类工作负载；候选必须满足稳定窗口后方可视为可达。

#### 场景: StatefulSet 候选筛选
- **当** 系统执行 StatefulSet L2 回滚候选筛选
- **那么** 系统必须从目标 StatefulSet 的历史版本中选择满足健康条件的候选

### 需求:回滚证据可观测输出
系统必须在每次 L2 回滚决策中输出候选筛选证据，并明确记录回滚执行动作与失败恢复结果；证据不足时必须明确说明降级原因。

#### 场景: StatefulSet 证据不足降级
- **当** StatefulSet 无法建立完整回滚证据链
- **那么** 系统必须禁止 L2 并记录降级到 L3 的依据

## REMOVED Requirements