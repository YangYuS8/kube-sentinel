## 1. API 与配置契约

- [x] 1.1 扩展 `api/v1alpha1/healingrequest_types.go`：新增快照策略字段（启用开关、保留窗口、恢复超时、每对象上限）与状态字段（快照标识、恢复结果、失败原因码）（验收：默认值/可选性/范围校验完整，失败路径可表达）。
- [x] 1.2 更新 `api/v1alpha1/healingrequest_types_test.go`：覆盖快照策略字段默认值、边界值、非法值与组合约束（验收：包含窗口/超时/上限的边界值测试）。
- [x] 1.3 同步 Helm 约束：更新 `charts/kube-sentinel/values.yaml` 与 `values.schema.json` 快照策略配置（验收：schema 与示例值可用，约束与 API 一致）。
- [x] 1.4 更新 `config/crd/_healingrequests.yaml` 并执行一致性检查（验收：`bash scripts/check-crd-consistency.sh` 通过）。

## 2. 持久快照能力实现

- [x] 2.1 新增持久快照实现（替代默认内存占位）：支持创建、查询、恢复、过期治理接口（验收：控制器重启后快照仍可用于恢复）。
- [x] 2.2 在 `internal/controllers/healingrequest_controller.go` 注入持久快照实现，保留内存实现用于测试替身（验收：生产路径不再依赖 `MemorySnapshotter`）。
- [x] 2.3 在 `internal/healing/orchestrator.go` 统一接入“快照→动作→恢复”流程到 Deployment/StatefulSet L1/L2（验收：快照失败阻断写操作并降级，回滚失败触发恢复）。
- [x] 2.4 实现快照幂等键与重复触发保护（验收：同一幂等窗口内重复触发不产生重复快照副作用）。
- [x] 2.5 实现快照生命周期治理（TTL/数量上限/清理策略）（验收：达到上限时按策略清理或阻断并有原因码）。

## 3. 安全门禁、可观测与告警

- [x] 3.1 将快照路径纳入维护窗口、速率限制、爆炸半径与熔断一致门禁（验收：命中门禁时仅只读评估+告警）。
- [x] 3.2 扩展审计与事件：记录快照创建、恢复执行、恢复结果与降级原因（验收：可按 workloadKind/phase/reason/snapshotId 检索）。
- [x] 3.3 扩展指标：新增快照创建成功率、恢复成功率、恢复时延、快照容量指标（验收：指标名称与标签稳定，单测可断言）。
- [x] 3.4 更新 `config/alerts/kube-sentinel-rules.yaml`：补充快照相关最小告警阈值（验收：失败率异常与恢复异常可触发告警）。

## 4. 验证与交付

- [x] 4.1 更新 `internal/healing/orchestrator_test.go` 与相关测试：覆盖快照创建失败阻断、恢复成功、恢复失败冻结、幂等阻断、容量上限（验收：关键失败路径均有单测）。
- [x] 4.2 更新 `scripts/drill-runtime-closed-loop.sh`：新增快照失败与恢复路径断言（验收：脚本输出快照证据链断言结果）。
- [x] 4.3 更新 `docs/release-and-rollback.md` 与 `docs/runtime-closed-loop-checklist.md`：补充快照灰度策略、回退手册与排障步骤（验收：文档包含风险、回滚、发布注意事项）。
- [x] 4.4 执行质量门禁：`go test ./...`、`go test -race ./internal/...`、`go vet ./...`、`golangci-lint run`（验收：全部通过）。
- [x] 4.5 执行 OpenSpec 严格校验：`openspec-cn validate --changes durable-snapshot-and-restore --strict`（验收：变更校验通过，可直接 `/opsx:apply`）。
