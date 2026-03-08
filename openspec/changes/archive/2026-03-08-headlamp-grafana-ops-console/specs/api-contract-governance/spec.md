## ADDED Requirements

### 需求:HealingRequest 必须暴露面向控制台的稳定状态语义

系统必须为 `HealingRequest` 暴露面向控制台消费的稳定状态语义，至少包括阶段、最近动作、失败或阻断原因、下一步建议和关联键；禁止让控制台只能依赖不可结构化的自由文本推断对象状态。

#### 场景: 控制台渲染对象摘要

- **当** Headlamp 或其他 Kubernetes 原生控制台渲染 `HealingRequest` 对象摘要
- **那么** 系统必须能够从稳定字段中直接读取阶段、最近动作、失败或阻断原因、下一步建议和关联键

### 需求:HealingRequest 必须提供适合列表视图的展示元数据

系统必须为 `HealingRequest` 提供适合通用 Kubernetes 控制台列表视图的展示元数据，例如打印列或等价 CRD 展示约束，使运维无需逐个展开对象详情即可识别关键运行态。

#### 场景: 控制台浏览 HealingRequest 列表

- **当** 运维在通用 Kubernetes 控制台中浏览 `HealingRequest` 列表
- **那么** 系统必须提供足以识别关键状态的列表展示元数据，而不要求用户逐个打开详情页才能完成基础筛查
