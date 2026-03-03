## ADDED Requirements

### 需求:L2 回滚目标选择规则
系统在执行 L2 回滚时必须以“最近一次健康运行的 Revision”为唯一候选基准，禁止默认回滚到“上一个 Revision”。

#### 场景: 存在多个历史 Revision
- **当** 当前 Deployment 需要触发 L2 回滚
- **那么** 系统必须按 Revision 时间逆序筛选健康记录，并选择最近满足健康判定的 Revision 作为回滚目标

### 需求:健康 Revision 判定可验证
系统必须基于可验证条件判定历史 Revision 的健康性，至少包含副本可用性、持续重启异常和关键告警状态。

#### 场景: 历史版本存在不完整观测数据
- **当** 某 Revision 缺少足够的健康观测证据
- **那么** 系统必须将其判定为不可用候选并继续查找下一个 Revision

### 需求:无健康 Revision 时的降级策略
系统在找不到健康 Revision 时必须禁止执行 L2 回滚，并升级到 L3 人工介入。

#### 场景: 所有历史 Revision 均不健康
- **当** 回滚候选遍历结束且无健康 Revision
- **那么** 系统必须终止自动回滚，更新状态为“等待人工介入”，并发送高优先级告警

## MODIFIED Requirements

## REMOVED Requirements
