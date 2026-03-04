## 为什么

当前项目已经具备“质量门禁可阻断”的交付能力，但尚未形成“可运营的稳定性闭环”：缺少统一 SLO 目标、门禁结果与值班动作之间的映射、以及事故处置中的证据与升级语义。随着功能面扩展到 Deployment 与 StatefulSet 分层策略，若没有 SLO 驱动的运行规范，系统会在“能阻断”与“能稳定运营”之间出现落差。

## 变更内容

- 新增运行稳定性 SLO 能力：定义最小 SLO 集、评估窗口、失败预算与越线动作语义。
- 建立门禁结果与事故响应联动：将 `allow/block/degrade` 结果映射为告警等级、值班通知和人工介入建议。
- 增加值班可诊断证据输出：统一输出门禁类别、根因、修复建议、恢复前置条件与推荐 Runbook。
- 扩展闭环演练矩阵：覆盖“门禁越线 -> 告警升级 -> 人工确认/回滚”的端到端路径断言。
- 明确灰度启用与回滚策略：先观测后放量，确保 SLO 越线时可立即降级到保守模式。

## 功能 (Capabilities)

### 新增功能
- `runtime-slo-governance`: 定义运行时 SLO 指标、预算与越线动作契约，约束交付与运行行为一致。

### 修改功能
- `runtime-production-hardening`: 增加门禁结果到事故响应等级的映射与恢复条件约束。
- `runtime-closed-loop-validation`: 增加 SLO 越线与事故升级路径的闭环验收断言。
- `delivery-quality-gates`: 增加质量门禁结果与 SLO 状态关联输出（用于值班与审计）。

## 影响

- 代码与脚本：`internal/observability/*`、`internal/safety/*`、`scripts/quality-gate.sh`、`scripts/drill-runtime-closed-loop.sh`。
- 文档与运行手册：`docs/release-and-rollback.md`、`docs/runtime-closed-loop-checklist.md` 需要补充 SLO 与值班流程。
- 指标与告警：Prometheus 规则与审计字段需增加 SLO 状态、预算消耗与升级级别。
- 交付流程：CI 通过不再是唯一准入条件，需结合 SLO 趋势与越线策略判定发布节奏。
