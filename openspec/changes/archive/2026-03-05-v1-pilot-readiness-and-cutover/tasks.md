## 1. Pilot 状态机与决策包

- [x] 1.1 定义 pilot/cutover 状态机与合法迁移表，输出 `pilot_prepare/pilot_observe/cutover_ready/cutover_blocked/cutover_done` 状态映射。
- [x] 1.2 增加单元测试：覆盖合法顺序迁移、非法跨阶段跳转与重复触发幂等。
- [x] 1.3 增加验收标准：非法跳转必须 `block`，合法迁移必须保留审计轨迹。
- [x] 1.4 定义 cutover decision pack 最小字段与稳定命名契约（decision/failureCategory/pilotBatch/rollbackTarget/traceKey/approvalLevel/timestamp）。
- [x] 1.5 增加单元测试：覆盖字段完整、字段缺失、字段语义冲突。
- [x] 1.6 增加验收标准：决策包字段缺失或语义冲突时必须阻断放量。

## 2. 放量门禁与自动回退

- [x] 2.1 实现 pilot 批次放量前置检查（质量结果、证据完整性、阶段合法性）。
- [x] 2.2 增加边界测试：前置检查失败、前置检查通过、证据过期。
- [x] 2.3 增加验收标准：前置检查任一失败必须阻断该批次。
- [x] 2.4 实现 pilot 观察窗口到 cutover 的单向门禁，禁止跨阶段直切。
- [x] 2.5 增加边界测试：观察窗口未完成、观察窗口达标、阶段重放幂等。
- [x] 2.6 增加验收标准：未完成观察窗口时禁止 cutover。
- [x] 2.7 实现 cutover 高优先级失败触发自动回退到上一稳定批次。
- [x] 2.8 增加单元测试：覆盖熔断触发、关键 SLO 越线、证据缺失三类自动回退。
- [x] 2.9 增加验收标准：触发回退条件时必须输出 `cutover_auto_rollback` 证据。

## 3. 值班交接、冻结窗口与 SLO 触发矩阵

- [x] 3.1 实现值班交接强制契约校验（handoffOwner/approvalLevel/traceKey/rollbackCommandRef/handoffTimestamp）。
- [x] 3.2 增加测试方案：交接字段完整、交接字段缺失、审批等级不匹配。
- [x] 3.3 增加验收标准：交接不满足时必须阻断切流。
- [x] 3.4 实现冻结窗口内人工覆盖阻断与窗口外受控覆盖规则。
- [x] 3.5 增加边界测试：窗口命中、窗口外、时间边界切换与重复覆盖幂等。
- [x] 3.6 增加验收标准：冻结窗口内禁止覆盖放量，仅允许只读评估。
- [x] 3.7 实现 pilot 期间 SLO 触发矩阵（observe_only/pause_rollout/rollback_required）。
- [x] 3.8 增加阈值边界测试：中度退化、严重退化、连续退化超窗口。
- [x] 3.9 增加验收标准：阈值语义在质量门禁/运行门禁/决策包中必须一致。

## 4. 交付收口与试运行演练

- [x] 4.1 更新质量门禁、发布文档和值班手册，明确 pilot 批次策略、cutover 失败处理与回退路径。
- [x] 4.2 执行最小回归：go test ./...、核心路径 race/vet/lint、质量门禁脚本。
- [x] 4.3 执行一次预生产 pilot 全链路演练并记录 cutover decision pack 样例与复盘结论。
