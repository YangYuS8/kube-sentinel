## 为什么

当前系统已经具备 SLO 治理、门禁三态和事故响应语义，但仍缺少“可持续运营”的落地机制：灰度启用策略不够细化、阈值调优节奏未标准化、值班告警抑制与复盘动作尚未形成稳定闭环。现在需要把已有能力从“可用”推进到“可运营、可迭代”。

## 变更内容

- 建立 SLO 灰度发布策略：按环境/命名空间分层启用，定义放量与回退条件。
- 固化阈值调优机制：明确初始阈值、观察窗口、变更审批与回滚规则。
- 完善值班联动：补齐最小告警集、抑制策略、升级路径与 runbook 约束。
- 增强复盘闭环：将“越线事件 -> 人工处置 -> 阈值调整”沉淀为可审计流程。
- 统一发布判定口径：使质量门禁、SLO 预算、事故响应三者在发布阶段语义一致。

## 功能 (Capabilities)

### 新增功能
- 无

### 修改功能
- `runtime-slo-governance`: 增加灰度启用与阈值调优治理要求，明确预算观察窗口与变更审批语义。
- `delivery-quality-gates`: 增加发布阶段的 SLO 语义一致性约束、告警抑制与值班输出规范。
- `runtime-closed-loop-validation`: 增加“灰度放量/回退 + 阈值调优 + 复盘动作”端到端演练断言。

## 影响

- 代码与脚本：`scripts/quality-gate.sh`、`scripts/drill-runtime-closed-loop.sh`、`scripts/drill_runtime_closed_loop_parser.go`。
- 观测与告警：`config/alerts/kube-sentinel-rules.yaml` 需体现分级告警与抑制策略。
- 运行文档：`docs/release-and-rollback.md`、`docs/runtime-closed-loop-checklist.md` 需补充灰度与调优流程。
- 交付流程：发布决策从“单点检查通过”升级为“门禁 + SLO + 响应链路一致通过”。
