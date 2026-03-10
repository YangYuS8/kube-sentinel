# ops-console-integration 规范

## ADDED Requirements

### 需求:运维控制台必须采用对象视图与指标视图分离模式

系统必须将 V1 运维界面分为对象视图与指标视图两类：对象视图必须优先通过 Headlamp 等 Kubernetes 原生控制台消费 `HealingRequest`、关联 workload 与 K8s Event；指标视图必须优先通过 Grafana 消费 Prometheus 指标；Agent 负责解释与交接，而不是引入新的高权限聚合控制台。

#### 场景: 运维查看单个 incident 详情

- **当** 运维需要排查某个 `HealingRequest` 的当前阶段、最近动作和阻断原因
- **那么** 系统必须允许其通过 Agent 获取摘要，通过对象视图查看 Kubernetes 资源，而不要求先经过自定义 UI 聚合层

#### 场景: 运维查看整体运行态趋势

- **当** 运维需要查看自愈触发、成功率、回滚、熔断或快照趋势
- **那么** 系统必须允许其通过 Grafana 等指标视图直接消费 Prometheus 指标完成查询

### 需求:对象视图与指标视图必须共享稳定关联键

系统必须为对象视图和指标视图提供稳定的关联键，至少包括 `correlationKey`、命名空间和 workload 标识，禁止让控制台联动完全依赖自由文本匹配。

#### 场景: 通过对象视图跳转定位指标

- **当** 运维从某个 `HealingRequest` 对象切换到指标视图进行进一步诊断
- **那么** 系统必须能够提供稳定关联键，以便在 Grafana 中按同一请求或 workload 范围继续定位

### 需求:控制台集成必须保持只读诊断优先

系统在 V1 控制台集成中必须以只读诊断和观测为主，禁止为了接入 Agent 或控制台而新增新的高权限写接口或绕过既有控制器/CRD 语义的操作面。

#### 场景: 接入 Agent 或控制台时评估写操作能力

- **当** 设计或实现 Agent 辅助入口、Headlamp 集成或 Grafana 集成
- **那么** 系统必须默认将其限制为解释或只读诊断入口，除非另有独立变更明确引入受控写路径

### 需求:本地与测试环境必须具备最小控制台启用入口

系统必须为本地和测试环境提供 Headlamp 与 Grafana 的最小启用说明或配置入口，使开发者能够在不建设独立 UI 后端的前提下完成对象视图和指标视图验证。

#### 场景: 在测试环境启用最小控制台

- **当** 开发者或测试人员在本地或测试集群中验证控制台接入
- **那么** 系统必须提供可重复执行的最小入口或文档，说明如何启用对象视图和指标视图

### 需求:V1 运维体验必须采用多入口协同模型
系统必须将 V1 运维体验定义为 Agent、Telegram、Headlamp、Grafana 和 kubectl 的协同模式；Telegram 负责主动到达，Agent 负责解释，Headlamp/Grafana/kubectl 负责对象、趋势和精确接管，禁止将独立厚控制台视为首发前置条件。

#### 场景: 运维处理单个 incident
- **当** 运维处理一个夜间 incident
- **那么** 系统必须允许其先在 Telegram 中查看 incident card，再通过 Agent、Headlamp、Grafana 或 kubectl 继续接手和排查

### 需求:Telegram 必须作为 V1 唯一主动通知通道
系统必须将 Telegram 定义为 V1 唯一主动通知通道，并要求该通道具备真实发送闭环，而不是仅停留在模板或文案定义层。

#### 场景: 单个 incident 进入主动通知路径
- **当** Agent 需要为某个 incident 主动通知值班人员
- **那么** 系统必须通过 Telegram 发送通知，而不得要求其他通知通道作为 V1 前置条件

#### 场景: 启用 Telegram 通知
- **当** 系统配置了 Telegram 发送能力并产生一个符合条件的 incident
- **那么** 系统必须实际向 Telegram 发送通知，而不是仅在本地生成消息模板

### 需求:Agent 通知必须区分短版和长版
系统必须为 Telegram 通知定义短版 ping 和长版 incident card 两层结构，禁止让夜间通知在第一条消息中同时承担唤醒和完整事故说明两种职责。

#### 场景: 发送 blocked incident 通知
- **当** 某个 incident 需要主动通知值班人员且当前状态为 blocked 或 manual follow-up
- **那么** 系统必须先发送可快速扫描的短版通知，并提供结构完整的长版 incident card
