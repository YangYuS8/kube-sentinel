## 1. StatefulSet 只读接入链路

- [x] 1.1 扩展接入映射与请求分流：识别 `workload_kind=StatefulSet` 并标记只读能力边界（验收：StatefulSet 请求可创建/更新且携带一致关联键与元数据）
- [x] 1.2 在编排入口增加 StatefulSet 只读策略判定，禁止 L1/L2 自动写操作（验收：StatefulSet 命中自动路径时统一阻断并返回可检索 reason）
- [x] 1.3 为接入与分流补充单元测试（Deployment/StatefulSet/非法 kind 三类）（验收：成功路径、拒绝路径、重复事件路径均覆盖）

## 2. 保守门禁与影子执行一致性

- [x] 2.1 复用保守门禁链路到 StatefulSet，只读阻断时输出 `shadowAction` 与人工介入建议（验收：status/event/audit 三处语义一致）
- [x] 2.2 扩展观测输出维度：补充 workloadKind 标签或字段，保证按工作负载检索（验收：指标、事件、审计可按 workloadKind 聚合）
- [x] 2.3 为保守门禁与影子执行补充失败路径测试（验收：证据不足、预算命中、观测降级三类路径可复现）
- [x] 2.4 补充边界值测试：幂等窗口、速率限制、爆炸半径在 StatefulSet 只读路径下行为一致（验收：临界点与越界点断言通过）

## 3. API/CRD/Chart 契约对齐

- [x] 3.1 评估并补齐 API/CRD 字段（如工作负载能力级别/状态原因码），保持向后兼容（验收：默认值、枚举、范围校验通过）
- [x] 3.2 同步 Helm `values.yaml` 与 `values.schema.json` 的 StatefulSet 接入开关与策略约束（验收：schema 拒绝非法配置且默认保持只读）
- [x] 3.3 更新运行文档与发布回滚文档（灰度、回滚、人工介入流程）（验收：发布评审可直接引用）
- [x] 3.4 为 API/Chart 改动补充回归测试（验收：序列化、默认值与兼容性断言通过）

## 4. 质量门禁与闭环验收

- [x] 4.1 执行并通过 `go test ./...`（验收：新增 StatefulSet 失败路径用例全部通过）
- [x] 4.2 执行并通过 `go test -race ./internal/...`（验收：controllers 与策略链路无竞态）
- [x] 4.3 执行并通过 `go vet ./...`、`golangci-lint run`、CRD 一致性检查（验收：任一失败阻断交付）
- [x] 4.4 更新闭环验收脚本/清单，纳入 StatefulSet 只读演练（验收：可输出分 workloadKind 的通过/失败报告）
