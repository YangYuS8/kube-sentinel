## 1. Deployment L2 编排落地

- [ ] 1.1 在 `internal/healing/orchestrator.go` 中引入 `processDeploymentL2` 并把 Deployment L1 失败分支接入真实 L2 流程；验收标准：L1 失败时不再写入 `not-allowed-in-mvp/skipped`，而是进入可解释的 L2 或 L3 结果；单元测试覆盖 L1 失败进入 L2 和 L1 成功仍保持原行为。
- [ ] 1.2 复用 `ListRevisions`、`SelectLatestHealthyRevision` 和 `ValidateRevisionDependencies` 实现 Deployment L2 候选选择与依赖校验；验收标准：最近健康 revision 会被记录到 status，候选缺失或依赖失败时降级为 L3 并输出原因码；单元测试覆盖无 revision、无健康候选、依赖校验失败三类失败路径。
- [ ] 1.3 将 Deployment L2 风险门禁接入 `DeploymentPolicy` 阈值与现有窗口评估逻辑；验收标准：超过阈值时阻断自动写操作并输出门禁快照，低于或等于阈值时允许继续；单元测试覆盖窗口为空、阈值临界值和超阈值边界。

## 2. 回滚性与可观测性对齐

- [ ] 2.1 实现 Deployment L2 回滚执行与快照恢复保护，统一成功、回滚失败且恢复成功、回滚失败且恢复失败三类状态迁移；验收标准：status 中必须反映 `deploymentL2Result`、`snapshotRestoreResult`、`nextRecommendation` 和 `lastHealthyRevision`；单元测试覆盖幂等窗口阻断、回滚成功、回滚失败恢复成功、回滚失败恢复失败。
- [ ] 2.2 对齐 Deployment L2 的事件、审计和指标输出，复用现有 metrics 计数器并清理旧的 MVP 占位语义；验收标准：不同结果会写入正确的 event reason、audit outcome 和 deployment L2 指标；单元测试覆盖 success、fallback、degraded 三类指标和关键事件原因。

## 3. 文档与质量门禁

- [ ] 3.1 更新发布/回滚与本地验证文档，说明 Deployment 已具备 L2 健康版本回滚、失败恢复和人工介入建议语义；验收标准：文档包含灰度启用注意事项、失败恢复说明和验证入口；检查项覆盖本地 smoke 与发布注意事项的一致性。
- [ ] 3.2 执行并固化与本变更相关的质量检查；验收标准：`go test ./...`、核心链路 `go test -race ./internal/... ./api/...`、`go vet ./...`、`golangci-lint run` 全部通过，且新增测试明确覆盖 L2 失败路径、幂等性和回滚性。
