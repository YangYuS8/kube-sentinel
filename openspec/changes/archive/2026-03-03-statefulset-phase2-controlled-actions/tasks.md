## 1. 设计与规范落地

- [x] 1.1 完成 `design.md`：明确 Phase 2 授权门禁、失败回退、灰度发布与风险缓解
- [x] 1.2 新增 capability `statefulset-controlled-healing` 的增量规范
- [x] 1.3 更新 `statefulset-conservative-onboarding`，补充阶段化能力与授权失败语义
- [x] 1.4 更新 `conservative-healing-policy`，定义 `conditional-writable` 与失败冻结
- [x] 1.5 更新 `deployment-healing-orchestration`，补充多工作负载统一编排与幂等约束
- [x] 1.6 更新 `runtime-production-hardening`，补充 Phase 2 灰度与指标门禁
- [x] 1.7 更新 `runtime-closed-loop-validation`，补充 Phase 2 验收矩阵与 CI 阻断

## 2. API 与配置面

- [x] 2.1 在 `api/v1alpha1/healingrequest_types.go` 增加 Phase 2 相关状态字段（授权结果、冻结态、失败原因）
- [x] 2.2 补充 `api/v1alpha1/healingrequest_types_test.go` 的序列化/默认值测试
- [x] 2.3 更新 `config/crd/_healingrequests.yaml` 并执行 CRD 一致性检查
- [x] 2.4 更新 `charts/kube-sentinel/values.yaml` 与 `values.schema.json`：新增开关、白名单、冻结窗口参数

## 3. 编排与策略实现

- [x] 3.1 扩展 `internal/healing/orchestrator.go`：引入 StatefulSet `conditional-writable` 判定流程
- [x] 3.2 在适配层补充 StatefulSet 受控动作接口与失败回退只读逻辑
- [x] 3.3 在保守门禁中实现四重授权校验（开关/白名单/审批/证据链）
- [x] 3.4 实现失败冻结窗口与人工解锁前阻断重复写操作
- [x] 3.5 增加幂等保护：同一窗口重复触发禁止二次副作用

## 4. 观测与审计

- [x] 4.1 扩展审计事件：记录授权判定、冻结触发、回退原因
- [x] 4.2 扩展指标维度：`workloadKind`、`actionType`、`decision`、`freezeState`
- [x] 4.3 增加 Phase 2 发布阻断指标（误动作率、回退率、冻结触发率）

## 5. 验证与交付

- [x] 5.1 补充/更新单元测试：授权通过、授权失败、动作失败回退、冻结阻断、幂等约束
- [x] 5.2 更新演练脚本 `scripts/drill-runtime-closed-loop.sh`：覆盖 StatefulSet Phase 2 全路径
- [x] 5.3 更新 `docs/release-and-rollback.md`：新增 Phase 2 灰度、阈值与回滚步骤
- [x] 5.4 执行 `go test ./...`、`go test -race ./...`、`go vet ./...`、`golangci-lint run`
- [x] 5.5 执行 `openspec-cn validate --change statefulset-phase2-controlled-actions --strict` 并修复问题
