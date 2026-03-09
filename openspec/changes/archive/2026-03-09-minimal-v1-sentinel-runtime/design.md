## 上下文

仓库当前已经同时包含 Deployment L1 闭环、Deployment L2/L3、StatefulSet 条件可写、发布治理、交付流水线、控制台接入与多类运行时治理能力。代码与规格都说明系统曾沿着“更强自动化”和“更强治理”方向生长，但新的 `openspec/config.yaml` 已经将产品身份重置为轻量值班哨兵。

本变更的设计重点不是增加新自动化，而是重新定义核心承诺：把 runtime 压缩为可解释、可审计、可人工接手的保守执行器，把复杂理解和交接能力迁移到 agent-facing 层，把发布治理和高风险自动化降级为非核心能力。

## 目标 / 非目标

**目标：**
- 将 V1 范围冻结为 Deployment 的最小安全闭环。
- 明确 runtime core、agent-facing capability、operator workflow 与 non-core experimental 的边界。
- 为 agent 补上最小产品职责：incident summary、next-step recommendation、handoff note。
- 明确产品表面采用 Agent + Headlamp + Grafana + kubectl 的多入口模式，而不是独立厚控制台。
- 为后续实现提供收缩方向，使代码和规格可以按新边界逐步对齐。

**非目标：**
- 不在本变更中重新设计完整的 agent 执行协议。
- 不在本变更中承诺 Deployment L2/L3 自动化恢复为核心能力。
- 不在本变更中承诺 StatefulSet 自动写路径进入 V1。
- 不在本变更中承诺独立 UI 后端、发布治理控制面或组织级 API 治理成为 V1 组成部分。

## 决策

### 决策 1：V1 runtime core 只承诺保守执行器

V1 runtime core 只包括：Webhook 接入、HealingRequest 统一状态、最小安全门禁、写前快照、Deployment 单一 L1 动作、基础审计/事件/指标输出。

原因：这些能力直接服务夜间值班减负，且可以保持低爆炸半径、幂等和可解释。继续把 L2/L3、StatefulSet 可写或发布治理放进核心，只会让 runtime 再次膨胀。

考虑过的替代方案：
- 保留 Deployment L2 作为默认核心：被拒绝，因为它会把“自动先试一次”重新变成“系统替人做复杂修复决策”。
- 同时保留 StatefulSet 条件可写：被拒绝，因为验证成本和误动作风险都明显高于 V1 目标。

### 决策 2：agent 先做解释层，而不是执行层

V1 的 agent-facing capability 先承诺 incident summary、next-step recommendation 和 handoff note；agent 默认只读，不成为生产写路径。

原因：当前产品真正缺的是夜间认知减负，而不是再多一层自动执行。解释层能力能显著提升值班体验，同时不会扩大 runtime 风险边界。

考虑过的替代方案：
- 先做 agent 驱动执行：被拒绝，因为会把开放式推理直接带入热路径。
- 先做 proposal/design/tasks 起草作为主能力：被拒绝，因为它更适合白天复盘，不是夜间第一波价值。

### 决策 3：产品表面采用多入口协同

V1 明确采用四类入口：Agent 负责解释，Headlamp 负责对象，Grafana 负责趋势，kubectl 负责精确查询与人工接管。

原因：当前仓库已经具备对象视图与指标视图的基础，继续建设独立厚控制台会重新把项目推回平台方向。多入口模式更轻量，也更贴近现有运维习惯。

考虑过的替代方案：
- 建设统一面板：被拒绝，因为需要自定义查询后端、读模型和更重的产品表面。
- 完全只保留 kubectl：被拒绝，因为它不能充分降低夜间解释成本。

### 决策 4：将复杂治理和实验自动化显式降级

Deployment L2/L3、StatefulSet 自动写路径、发布就绪度、pilot/cutover、交付流水线治理、组织级 API 兼容性治理与独立厚控制台，全部从 V1 核心承诺中移出，降级为 non-core experimental 或 operator workflow。

原因：这些能力并非没有价值，而是其主要价值发生在白天治理和平台演进阶段，不应继续定义当前产品身份。

考虑过的替代方案：
- 保留这些能力但弱化文案：被拒绝，因为模糊承诺会继续拖大范围。

## 风险 / 权衡

- [风险] 范围收缩后，看起来像“回退能力” → 缓解措施：将非核心能力显式保留为 experimental/workflow，说明是降级承诺而非否定已有探索。
- [风险] 现有规格仍然覆盖大量非核心能力，短期内会出现“新世界观”和旧 spec 并存 → 缓解措施：本变更通过 proposal/spec/task 先建立迁移锚点，后续逐步归档或降级旧能力。
- [风险] 只保留 Deployment L1 可能被认为自动化价值不足 → 缓解措施：同步引入 agent 的解释和交接能力，强调“少做但说清楚”的产品价值。
- [风险] Agent 作为解释层后，产品边界横跨 runtime 与非 runtime 资产 → 缓解措施：要求 agent 输出结构化、可审计、可追溯，并保持默认只读。

## Migration Plan

1. 先以规格方式冻结 V1 边界，明确核心承诺与非核心能力分类。
2. 以新规格为准，逐步调整 README、docs 和实现优先级，使外部叙事先收缩。
3. 后续实现阶段优先补齐 agent-facing 最小能力与 runtime core 清理，再处理旧 spec 的归类、归档或降级。
4. 若后续发现收缩过度，可在新的独立变更中重新引入某项能力，但不得绕过本次边界定义直接回流到 V1 核心。

## Open Questions

- Agent 的最小输出载体应该绑定在 `HealingRequest.status`、审计事件、独立工件还是外部接口？
- V1 是否需要在文档层正式定义“只读模式”和“最小自动模式”两种安装/运行配置档？
- 现有非核心 spec 是统一归档，还是保留但显式标注 experimental，更利于后续演进？
