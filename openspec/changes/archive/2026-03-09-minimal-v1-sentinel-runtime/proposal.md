## 为什么

当前仓库已经同时承载了夜间值班哨兵、运行时治理、发布治理和实验性自动化能力，核心产品边界变得模糊。现在需要把 V1 明确收缩为一个轻量、保守、可人工接管的值班工具，确保真正承诺的能力聚焦在 Deployment 的最小安全闭环与清晰的人工接力体验上。

## 变更内容

- 将 V1 产品范围冻结为面向夜间值班的轻量哨兵，而不是自治运维平台。
- 将 runtime core 限定为 Alertmanager 接入、HealingRequest 统一状态、最小安全门禁、写前快照、Deployment 单一 L1 自动动作，以及基础审计/事件/指标输出。
- 引入 agent-facing 的最小能力边界：incident summary、next-step recommendation、handoff note，以及跨 Agent、Headlamp、Grafana、kubectl 的稳定关联语义。
- 明确运维体验采用多入口协同：Agent 负责解释，Headlamp 负责对象，Grafana 负责趋势，kubectl 负责精确查询与人工接管。
- 将 Deployment L2/L3 自动化、StatefulSet 自动写路径、发布治理、组织级 API 治理和独立厚控制台从 V1 核心承诺中移出，降级为非核心或实验能力。

## 功能 (Capabilities)

### 新增功能
- `agent-incident-assistance`: 定义面向值班场景的 agent 最小能力边界，包括事件摘要、下一步建议、交接说明和跨入口关联语义。

### 修改功能
- `deployment-safe-l1-mvp`: 将 V1 承诺进一步收缩到 Deployment 单一 L1 自动动作与最小安全闭环。
- `alertmanager-webhook-ingestion`: 明确接入层只需为 Deployment L1 和只读非核心路径提供最小可执行上下文。
- `ops-console-integration`: 明确 V1 不建设独立厚控制台，采用 Agent + Headlamp + Grafana + kubectl 的多入口协同模式。
- `api-contract-governance`: 收缩核心状态契约，只保留 incident 对象最小稳定状态语义与列表展示元数据要求。
- `observability`: 将值班理解与人工接管所需的基础审计、事件、指标与关联键定义为 V1 核心观察面。
- `local-deployment-and-dev-loop`: 将部署与验证体验定义为工具化入口，要求快速安装、快速验证和快速退出。

## 影响

- 受影响的代码主要集中在 `internal/ingestion`、`internal/controllers`、`internal/healing`、`internal/safety`、`internal/observability` 和 `api/v1alpha1`。
- 受影响的产品表面包括 `HealingRequest` 状态语义、最小安装脚本、Headlamp/Grafana 接入说明以及未来的 agent 输出契约。
- 受影响的规格体系包括 V1 范围、接入、观察面、状态契约和部署采用路径；多阶段自动化与发布治理能力将被明确降级为非核心能力。
