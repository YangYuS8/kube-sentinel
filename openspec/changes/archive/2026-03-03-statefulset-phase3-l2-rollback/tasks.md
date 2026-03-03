## 1. API 与配置扩展

- [x] 1.1 扩展 `api/v1alpha1/healingrequest_types.go`：新增 StatefulSet L2 候选证据、回滚结果、降级建议、冻结协同字段（验收：字段具备默认值/可选性/校验约束，失败路径字段可表达）。
- [x] 1.2 更新 `api/v1alpha1/healingrequest_types_test.go`：覆盖新增字段默认值、边界值与校验失败路径（验收：包含 freezeWindow、候选窗口、开关组合边界测试）。
- [x] 1.3 更新 `charts/kube-sentinel/values.yaml` 与 `values.schema.json`：新增 StatefulSet L2 开关、候选窗口、冻结时长、灰度阈值配置（验收：schema 约束完整且示例值可用）。
- [x] 1.4 更新 `config/crd/_healingrequests.yaml` 并执行 CRD 一致性检查（验收：`bash scripts/check-crd-consistency.sh` 通过）。

## 2. 编排与适配实现

- [x] 2.1 扩展 `internal/healing/orchestrator.go`：实现 StatefulSet L1 失败后进入 L2 判定与 L3 降级状态机（验收：L1→L2→L3 路径可复现，状态迁移可解释）。
- [x] 2.2 扩展适配层：实现 StatefulSet 历史版本枚举、健康候选筛选与依赖完整性校验接口（验收：无候选/依赖缺失时禁止 L2 并输出证据）。
- [x] 2.3 实现 L2 执行前快照与失败恢复：回滚失败自动 restore 并进入只读冻结（验收：失败路径具备回滚性测试）。
- [x] 2.4 实现 L2 幂等保护：同一幂等窗口内禁止重复回滚副作用（验收：重复触发测试显示阻断并有原因码）。

## 3. 安全门禁与观测

- [x] 3.1 扩展安全门禁：维护窗口、速率限制、爆炸半径与熔断在 L2 路径一致生效（验收：边界值测试覆盖命中与放行两类结果）。
- [x] 3.2 扩展审计与事件：记录候选筛选依据、回滚动作、恢复结果、降级原因（验收：可按 workloadKind/phase/reason 检索）。
- [x] 3.3 扩展指标：新增 StatefulSet L2 成功率、失败回退率、冻结触发率、降级率（验收：指标名称与标签稳定、单测可断言）。

## 4. 验证与交付

- [x] 4.1 更新 `internal/healing/orchestrator_test.go` 与相关测试：覆盖 L2 正向、无候选降级、依赖缺失、执行失败恢复、冻结阻断、幂等阻断（验收：关键失败路径均有测试）。
- [x] 4.2 更新 `scripts/drill-runtime-closed-loop.sh`：新增 StatefulSet Phase 3 演练断言（L1 失败转 L2、L2 失败转 L3）（验收：脚本输出三阶段断言结果）。
- [x] 4.3 更新 `docs/release-and-rollback.md`：补充 StatefulSet L2 灰度步骤、阈值与一键回退策略（验收：文档包含风险与回滚手册）。
- [x] 4.4 执行质量门禁：`go test ./...`、`go test -race ./internal/...`、`go vet ./...`、`golangci-lint run`（验收：全部通过）。
- [x] 4.5 执行 OpenSpec 严格校验：`openspec-cn validate --changes statefulset-phase3-l2-rollback --strict`（验收：变更校验通过，可直接 `/opsx:apply`）。