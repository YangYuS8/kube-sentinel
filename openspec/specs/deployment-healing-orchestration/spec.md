## ADDED Requirements

### 需求:v1alpha1 Deployment 自愈编排范围
系统必须在 v1alpha1 阶段仅对 Deployment 执行自动自愈动作，禁止对 StatefulSet 等其他工作负载直接执行写操作。

#### 场景: 非 Deployment 目标被触发
- **当** 告警事件指向的目标资源类型不是 Deployment
- **那么** 系统必须拒绝执行自愈写操作，并记录“类型不在 v1alpha1 支持范围”的审计事件

### 需求:Workload 适配接口预留
系统必须提供 Workload 适配接口以隔离对象差异，编排层禁止直接依赖具体工作负载实现细节。

#### 场景: 控制器执行策略链路
- **当** Reconciler 进入策略执行阶段
- **那么** 系统必须通过适配接口访问对象特定能力（如 Revision 查询、回滚执行），而不是内联对象分支逻辑

### 需求:编排链路幂等执行
系统必须保证同一告警在重复触发时不会导致重复副作用，必须以目标对象当前状态作为判定依据。

#### 场景: 同一告警重复到达
- **当** 在幂等窗口内接收到同一 Deployment 的重复告警
- **那么** 系统必须复用当前策略阶段与执行记录，禁止重复创建冲突动作

## MODIFIED Requirements

## REMOVED Requirements
