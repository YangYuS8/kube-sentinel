## 为什么

当前能力已能完成自愈与门禁判定，但“首个生产版本上线”仍缺少一条可重复执行的试运行与切流闭环：谁批准、何时放量、失败如何自动降级、何时强制回退还未被统一约束。现在需要把这些运行策略收敛为 V1 Pilot 的最小上线路径，降低首发风险并提升值班一致性。

## 变更内容

- 增加 V1 Pilot 试运行策略：按固定批次进行灰度切流，定义每批次放量前置条件与观察窗口。
- 增加自动 cutover 判定：当关键稳定性与安全门禁满足时，允许从 pilot 灰度推进到全量；否则自动停留或回退。
- 增加 fail-safe 回退触发矩阵：将 SLO 退化、熔断触发、维护窗口冲突、审计缺失映射为明确回退动作。
- 增加 oncall 交接与运行手册最小契约：确保每次 pilot 操作具备审批记录、关联键、回退命令与复盘条目。
- 统一 V1 上线验收输出：生成机器可读 cutover decision pack 与人类可读值班摘要。

## 功能 (Capabilities)

### 新增功能

- `v1-pilot-readiness-and-cutover`: 定义首个可上线版本的试运行、切流与回退闭环规范。

### 修改功能

- `delivery-quality-gates`: 增加 pilot 批次放量前置检查与 cutover 决策字段要求。
- `release-readiness-and-oncall-automation`: 增加值班交接、审批层级、运行手册与回退执行契约。
- `runtime-production-gating`: 增加 pilot->cutover 的分阶段门禁与自动阻断/回退语义。
- `runtime-slo-governance`: 增加 pilot 期间的 SLO 退化触发与回退阈值规则。

## 影响

- 主要影响交付与运行治理脚本、发布流程文档、值班审计数据契约。
- 不新增核心自愈动作类型，不扩展 CRD API 面向用户的行为边界。
- CI/质量门禁将增加与 pilot/cutover 相关的验证步骤与证据输出。
