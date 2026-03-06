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
