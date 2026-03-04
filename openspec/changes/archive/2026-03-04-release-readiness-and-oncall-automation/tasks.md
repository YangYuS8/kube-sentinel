## 1. 发布就绪摘要数据模型与聚合接口

- [x] 1.1 在 `internal/observability` 增加发布就绪摘要结构体与序列化输出（含 actionType、riskLevel、strategyMode、circuitTier、operatorOverride、rollbackCandidate、openIncidents、recentDrillScore）。
- [x] 1.2 在聚合逻辑中实现“单一来源摘要生成”入口，确保相同输入生成稳定、幂等的摘要结果。
- [x] 1.3 为摘要聚合增加单元测试：覆盖正常输入、字段缺失、未知动作类型与空 incident 列表的失败路径与降级行为。

## 2. 值班自动化映射与动作模板

- [x] 2.1 在 `internal/observability` 增加值班映射规则：将 `allow/degrade/block` 决策映射到标准化 on-call action template。
- [x] 2.2 增加人工覆盖（operator override）记录与展示字段，确保摘要中可追踪“谁在何时覆盖了什么决策”。
- [x] 2.3 为值班映射增加单元测试：覆盖 allow/degrade/block 三条主路径、未知决策值、重复输入幂等场景。

## 3. 演练与事件归并增强

- [x] 3.1 在运行时闭环验证路径中实现 drill 成绩聚合字段（successRate、rollbackLatencyP95、gateBypassCount）并纳入摘要。
- [x] 3.2 增加 incident 去重与关联逻辑对摘要输出的影响验证，保证演练/真实事件可区分并可追踪。
- [x] 3.3 为演练与事件归并增加单元测试：覆盖零样本窗口、边界时间窗口（临界分钟）、重复事件输入与失败回退路径。

## 4. 质量门禁与生产门禁对接

- [x] 4.1 扩展 `scripts/quality-gate.sh` 输出发布就绪证据块（release readiness evidence），并在缺失关键字段时返回非零退出码。
- [x] 4.2 更新生产门禁判定逻辑：当摘要缺少 rollbackCandidate 或 openIncidents 超阈值时强制 `block`。
- [x] 4.3 为门禁脚本与判定逻辑增加测试：覆盖阈值边界、字段缺失、人工覆盖存在/不存在的通过与阻断路径。

## 5. 可观测性指标与文档同步

- [x] 5.1 在 `internal/observability/metrics.go` 增加发布就绪相关指标（summary generation total、staleness、override count）并接入现有标签体系。
- [x] 5.2 更新运行手册与发布文档，补充“发布就绪摘要读取”“值班动作模板执行”“人工覆盖审计”操作步骤。
- [x] 5.3 为指标与文档关联行为增加验证：单元测试覆盖标签边界值、高基数防护；脚本检查确保文档中的证据字段与实际输出一致。

## 6. Helm 与配置约束同步

- [x] 6.1 为新增发布就绪与值班自动化配置项更新 `charts/kube-sentinel/values.yaml` 默认值，并在 `values.schema.json` 增加约束。
- [x] 6.2 为 Helm schema 约束增加测试：覆盖必填项、枚举边界、非法类型与默认值回填行为。
- [x] 6.3 增加一致性校验任务：确保 API/脚本/Helm 中同名策略字段语义一致，若不一致则质量门禁失败。

## 7. 收尾验收与回滚演练

- [x] 7.1 执行针对本变更的最小回归测试集（observability、ingestion、safety 相关），并记录失败路径处置结果。
- [x] 7.2 执行一次端到端 dry-run：生成发布就绪摘要→触发门禁→输出值班模板→审计人工覆盖，验证全链路可追踪。
- [x] 7.3 进行回滚性与幂等性验收：重复执行同一输入不产生副作用；回滚后门禁与摘要恢复到预期状态。
