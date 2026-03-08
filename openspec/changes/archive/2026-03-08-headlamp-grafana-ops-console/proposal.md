## 为什么

当前 Kube-Sentinel 已经具备可被界面消费的两类核心信息源：Kubernetes 原生对象状态和 Prometheus 指标，但项目仍缺少一个低开发量、低耦合的运维界面方案。继续直接建设独立前后端 UI，会过早引入查询聚合层、认证授权边界和审计持久化问题，偏离当前“先交付可验证控制面”的阶段目标。

现在推进 Headlamp + Grafana 优先的控制台集成，能够先用 Kubernetes 原生资源视图承接对象排障，用指标面板承接运行态总览，在不改造核心控制器链路的前提下尽快形成“对象详情 + 趋势看板”的最小运维界面。

## 变更内容

- 新增一项面向运维控制台的集成能力，明确 Headlamp 负责对象视图、Grafana 负责指标视图的分工，并定义最小接入范围。
- 为 HealingRequest 资源补齐更适合控制台消费的展示语义，例如列表可读性、关键状态字段与关联键约束。
- 为 Grafana 接入补齐最小可抓取的指标暴露契约与面板输入约束，避免额外维护仅供展示的查询模型。
- 收敛部署与文档入口，明确如何在本地或测试环境中启用 Headlamp 与 Grafana 视图，而不把它们扩展成新的独立业务后端。
- 明确非目标：不在本变更中建设自定义前端门户，不引入新的持久化审计读模型，也不修改现有自愈核心决策链路。

## 功能 (Capabilities)

### 新增功能

- `ops-console-integration`: 定义 Kube-Sentinel 基于 Headlamp 与 Grafana 的最小运维控制台集成能力，包括对象视图、指标视图、关联键与部署入口。

### 修改功能

- `observability`: 增加供 Grafana 消费的指标暴露、面板分组与最小 dashboard 输入约束。
- `api-contract-governance`: 增加供控制台消费的状态字段、展示语义与对象关联键稳定性约束。
- `local-deployment-and-dev-loop`: 增加本地/测试环境中启用 Headlamp 与 Grafana 所需的最小入口与说明。

## 影响

- 受影响代码：CRD 展示元数据、指标暴露方式、Helm/chart 或安装清单中的观测接入配置、README/运维文档。
- 受影响系统：Headlamp、Grafana、Prometheus 抓取链路以及本地/测试环境的运维入口。
- 风险控制：必须保持“优先复用 K8s 资源视图与 Prometheus 指标”的边界，避免变更范围滑向独立 UI 后端建设。
