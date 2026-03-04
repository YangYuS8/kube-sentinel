## 为什么

当前主线已具备门禁、SLO 和闭环演练能力，但“API 契约一致性与兼容性治理”仍缺少系统化收口：CRD 字段约束、版本兼容策略、status 语义最小集合与 Helm 约束校验之间尚未形成统一交付契约。随着变更频率上升，这会放大发布回归风险并削弱可持续交付的稳定性。

## 变更内容

- 新增 API 契约治理能力：统一定义 CRD 字段默认值、可选性与校验约束的规范级要求。
- 新增 API 兼容性策略：为 API 变更定义向后兼容、迁移路径与版本演进规则。
- 新增 status 语义最小集合：要求状态字段稳定输出“策略阶段、最近动作、失败原因、下一步建议”。
- 将 API/CRD/Helm 一致性校验纳入交付门禁证据输出，确保发布前可审计。
- 修改生产门禁规范目的与发布判定证据要求，使其与 API 契约治理形成闭环。

## 功能 (Capabilities)

### 新增功能
- `api-contract-governance`: 定义 API 契约、兼容性与状态语义的统一规范，约束 CRD 与 Helm 的一致性交付。

### 修改功能
- `delivery-quality-gates`: 增加 API/CRD/Helm 一致性证据与兼容性阻断规则。
- `runtime-production-gating`: 补齐规范目的与发布判定证据字段，绑定 API 契约风险判定。

## 影响

- OpenSpec：新增 `api-contract-governance` 主规范，并补充 `delivery-quality-gates`、`runtime-production-gating` 增量规范。
- API/CRD：`api/v1alpha1` 字段约束与状态语义将进入统一治理要求。
- 交付流程：质量门禁与 CI 需输出更完整的 API 契约证据。
- 运维发布：发布前检查与回滚决策将增加兼容性与状态语义维度。