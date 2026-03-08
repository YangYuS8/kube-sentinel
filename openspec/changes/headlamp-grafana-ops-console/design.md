## 上下文

当前 Kube-Sentinel 已经具备两类适合被运维界面直接消费的数据面：一类是 Kubernetes 原生资源，尤其是 `HealingRequest` 的 `status` 语义；另一类是 Prometheus 指标。项目仍未形成统一的控制台接入方案，导致对象排障与运行总览分散在 kubectl、日志和指标端点之间。

按照当前项目阶段，如果直接建设独立前后端门户，会立刻引入新的查询聚合层、认证授权边界和审计持久化需求，明显高于现阶段“先形成最低成本可用界面”的目标。因此本设计选择以 Headlamp 承接对象视图、Grafana 承接指标视图，并把两者都建立在已有 Kubernetes API 与 Prometheus 指标之上。

## 目标 / 非目标

**目标：**

- 为 Kube-Sentinel 定义最小可用的运维控制台集成方案：Headlamp 查看对象，Grafana 查看趋势。
- 让 `HealingRequest` 资源具备更适合 Headlamp 列表和详情展示的稳定语义与关联字段。
- 让控制器指标能够以标准 Prometheus 抓取方式接入 Grafana，而不新增自定义统计后端。
- 为本地和测试环境提供最小的 Headlamp/Grafana 启用与验证说明。

**非目标：**

- 不建设独立的 Web 前端门户或新的后端查询 API。
- 不在本变更中解决完整审计时间线持久化或全文检索。
- 不改变现有自愈决策链路、门禁判定语义或 CRD 业务能力边界。
- 不把 Headlamp 或 Grafana 扩展成可写控制平面，首版以只读诊断和观测为主。

## 决策

### 决策 1：Headlamp 作为对象视图入口，直接消费 Kubernetes 资源

- 选择：将 Headlamp 作为 `HealingRequest`、关联 workload、K8s Event 的对象视图入口，不新增专门给 UI 用的查询 API。
- 原因：当前最有价值的对象信息已经位于 CRD `status` 和标准 K8s 元数据中；直接复用 Kubernetes 资源视图开发量最小，也符合既有 `ui_integration` 约束。
- 备选：自定义 UI 后端虽然能做更自由的聚合，但会提前引入认证、聚合查询和持久化设计，当前不划算。

### 决策 2：Grafana 作为趋势与系统总览入口，直接消费 Prometheus 指标

- 选择：将 Grafana 用于展示触发量、成功率、L1/L2 结果、快照与门禁趋势，数据源直接来自 Prometheus 抓取的 `kube_sentinel_*` 指标。
- 原因：项目已经有较完整的指标面，Grafana 适合承接趋势和聚合视图；不需要在控制器中再维护一个仅供展示的读模型。
- 备选：在控制器中额外维护 dashboard 查询对象会扩大实现面，并与现有指标系统重复。

### 决策 3：两类控制台通过稳定关联键串联，而不是通过新聚合服务强绑定

- 选择：用 `correlationKey`、命名空间、workload 名称作为对象视图和指标视图的共享定位信息。
- 原因：这能在不引入自定义中间层的情况下形成最小联动，也与现有 `HealingRequest.status` 语义一致。
- 备选：新增单独聚合对象或专门的 UI 索引服务，会显著抬高范围。

### 决策 4：Grafana 接入采用 Kubernetes 原生监控发现路径

- 选择：为控制器 metrics 暴露标准可抓取入口，并优先通过 Service/Monitor 类配置接入 Prometheus 生态，而不是要求用户手工拼接抓取规则。
- 原因：这样更适合 Helm/chart 和测试环境部署，也更符合“低摩擦启用”的目标。
- 备选：仅依赖 README 手工配置抓取虽然能用，但不可重复，也不利于测试环境标准化。

### 决策 5：对象视图可读性优先通过 CRD 展示元数据与稳定状态字段改进

- 选择：通过 `HealingRequest` 的打印列、状态字段稳定性和关键字段命名，提升 Headlamp/通用 K8s 控制台的可读性。
- 原因：这比先写 Headlamp 插件成本更低，也更符合 CRD-first 的项目形态。
- 备选：直接写专用插件虽然更灵活，但会让项目绑定特定控制台生态，且增加维护面。

## 风险 / 权衡

- [风险] Headlamp 零插件模式下，列表体验可能仍不够理想
  → 缓解：优先补齐 CRD 打印列和稳定字段语义，把插件开发留到后续阶段。

- [风险] Grafana 依赖 Prometheus 抓取链路，若指标暴露入口不标准会增加部署成本
  → 缓解：在本变更中把 metrics 暴露和最小监控配置一并定义清楚。

- [风险] `correlationKey` 如果未稳定贯穿对象和指标，会降低跨界面联动效果
  → 缓解：将其纳入 API/状态语义要求，而不是作为实现细节散落在代码里。

- [风险] 用户可能误以为本变更会交付完整自定义门户
  → 缓解：在 proposal、规范和任务中明确把自定义 UI、审计持久化和复杂聚合能力列为非目标。

## Migration Plan

1. 先定义并实现 `HealingRequest` 的最小控制台可读性增强，包括打印列和稳定状态字段。
2. 补齐 metrics 暴露入口与 Prometheus/Grafana 接入配置，使测试环境可重复启用仪表盘。
3. 提供最小 dashboard 分组与文档说明，形成对象视图与指标视图的基本联动。
4. 在本地和测试环境先验证 Headlamp 列表/详情展示与 Grafana 面板查询，再决定是否需要后续插件化增强。
5. 回滚策略：若控制台接入引入过高运维复杂度，可保留 CRD/status 语义改进，暂时移除监控配置模板与文档入口，不影响控制器主链路。

## Open Questions

- Grafana 面板是以内置 JSON/ConfigMap 方式提供，还是先只提供查询模板与导入说明？
- Prometheus 接入是优先提供 `ServiceMonitor`、`PodMonitor`，还是两者都提供但以一方为默认？
- 首版是否需要为 Headlamp 预留插件扩展点，还是只依赖通用 CRD 视图即可？
