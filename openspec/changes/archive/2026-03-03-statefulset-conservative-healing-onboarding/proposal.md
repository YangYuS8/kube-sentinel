## 为什么

当前 Kube-Sentinel 已在 Deployment 路径上完成保守策略收敛，但与项目目标中“Deployment/StatefulSet 为核心对象”仍存在缺口。直接开放 StatefulSet 自动写操作风险较高，因此需要先建立可观测、可审计、可回退的只读接入基线。

## 变更内容

- 新增 StatefulSet 保守接入能力：支持事件接入、门禁评估、证据链计算、影子执行输出与人工介入建议。
- 明确 v1 阶段对 StatefulSet 仅允许只读评估与告警，不执行 L1/L2 写操作。
- 扩展运行态验证与可观测输出，确保 StatefulSet 在阻断路径与 Deployment 一致可检索。
- 为后续 Phase 2（受控自动动作）预留兼容字段与迁移路径，不在本次实现中启用。

## 功能 (Capabilities)

### 新增功能

- `statefulset-conservative-onboarding`: 定义 StatefulSet 在保守模式下的只读接入契约、状态语义、证据与输出要求。

### 修改功能

- `alertmanager-webhook-ingestion`: 扩展接入层对 StatefulSet 事件的规范化映射与只读流程分流。
- `runtime-closed-loop-validation`: 调整闭环验证范围，覆盖 StatefulSet 只读路径断言与关联键一致性。
- `runtime-production-hardening`: 增补多工作负载场景下的门禁输入、降级策略与观测一致性要求。
- `conservative-healing-policy`: 补充“StatefulSet 默认只读阻断”的保守策略约束。

## 影响

- 代码影响：`internal/ingestion`、`internal/healing`、`internal/observability`、`api/v1alpha1` 与相关测试。
- API/CRD 影响：可能新增工作负载策略字段与状态枚举的兼容扩展（保持向后兼容）。
- 运维影响：Prometheus 指标与 K8s Event 的标签维度可能增加 workloadKind 细分。
- 发布影响：新增灰度开关，默认关闭 StatefulSet 自动写操作，仅启用只读评估。
