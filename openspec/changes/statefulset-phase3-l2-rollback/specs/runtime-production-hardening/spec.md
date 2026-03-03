## ADDED Requirements

### 需求:StatefulSet L2 灰度发布控制
系统必须支持 StatefulSet L2 的灰度发布控制，至少包含命名空间白名单、审批开关、快速回退开关与冻结阈值。

#### 场景: L2 灰度范围受控
- **当** 仅对白名单命名空间开启 StatefulSet L2
- **那么** 非白名单命名空间必须继续保持 L1+L3 路径

### 需求:StatefulSet L2 发布阻断指标
系统必须输出 StatefulSet L2 成功率、失败回退率、冻结触发率与降级率，并作为发布阻断依据。

#### 场景: 指标越线阻断
- **当** 任一 L2 关键指标超过阈值
- **那么** 系统必须自动降级并提示回退到保守模式

## MODIFIED Requirements

### 需求:执行链路去硬编码
系统必须将 StatefulSet L2 候选窗口、冻结时长、灰度阈值与降级条件全部配置化，禁止硬编码策略值。

#### 场景: L2 参数读取
- **当** 系统执行 StatefulSet L2 判定与执行
- **那么** 必须从声明式配置读取全部 L2 控制参数

## REMOVED Requirements