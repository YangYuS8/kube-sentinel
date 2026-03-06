## 目的

定义 Alertmanager Webhook 事件到 HealingRequest 的标准接入与分流规则，确保 Deployment 与 StatefulSet 在上下文映射、幂等去重和能力边界上具备一致且可审计的行为。

## 需求

### 需求:多工作负载事件分流

系统必须在 Webhook 接入阶段识别 `workload_kind` 并按能力边界分流处理。

#### 场景: StatefulSet 事件分流

- **当** 接入事件的 `workload_kind` 为 `StatefulSet`
- **那么** 系统必须将请求标记为只读路径并附带对应阻断语义

### 需求:Webhook 事件接入

系统必须接收 Alertmanager Webhook 事件并映射为可追踪上下文；当 `workload_kind` 为 StatefulSet 时，必须保留同等上下文信息并显式标记“只读评估”。

#### 场景: StatefulSet 上下文映射

- **当** 系统创建或更新 HealingRequest
- **那么** 必须写入与 Deployment 一致的关联键、告警元数据与工作负载类型标记

### 需求:事件幂等去重

系统必须对重复告警事件执行幂等去重，且去重窗口必须由请求配置项驱动；对 StatefulSet 只读路径同样生效。

#### 场景: StatefulSet 去重窗口生效

- **当** StatefulSet 事件在幂等窗口内重复到达
- **那么** 系统必须判定为重复并跳过重复处理

## ADDED Requirements

### 需求:Deployment L1 首发接入基线

系统必须在 Alertmanager Webhook 接入阶段为 Deployment 首发 L1 闭环写入最小可执行上下文，至少包含幂等键、工作负载标识、告警类别、告警严重级别和后续编排所需关联键。

#### 场景: Deployment 告警映射首发上下文

- **当** 接入事件的 `workload_kind` 为 `Deployment`
- **那么** 系统必须创建或更新可驱动 L1 编排的 HealingRequest，并保留首发闭环所需的最小上下文元数据

### 需求:首发范围外工作负载保持只读

系统必须在首发版本中将非 Deployment 自动写能力保持为只读评估路径，禁止因为首发交付而默认放宽其他工作负载的自动动作权限。

#### 场景: 非 Deployment 告警进入首发版本

- **当** 接入事件不属于 Deployment L1 自动处置范围
- **那么** 系统必须保留只读阻断语义或拒绝创建写路径上下文
