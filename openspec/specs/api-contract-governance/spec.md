# api-contract-governance 规范

## 目的

定义 Kube-Sentinel 极简 V1 中 incident 对象的最小状态契约，确保 `HealingRequest` 对 Agent、Headlamp 与人工接管都保持稳定、可读、可安装。

## 需求

### 需求:CRD 字段契约完整性

系统必须为对外暴露的 CRD 字段声明默认值、可选性和校验约束（枚举、范围或正则），并必须保证生成出的 CRD 清单与这些声明保持一致且可直接安装，禁止无约束或不可安装的生成物进入生产发布流程。

#### 场景: 字段缺失约束被阻断

- **当** API 变更引入或修改 CRD 字段但缺少默认值或校验约束
- **那么** 系统必须阻断交付并输出缺失字段清单

#### 场景: 生成物元数据错误被阻断

- **当** 生成出的 HealingRequest CRD 清单缺少正确的 group、version 或资源名称，或无法直接安装到集群
- **那么** 系统必须阻断交付并输出生成链路缺陷

### 需求:状态语义最小集合

系统必须在状态输出中稳定反映 incident 阶段、最近动作、失败原因或阻断原因、最近门禁裁决和下一步建议，并保留适合 Agent、Headlamp 和人工接管共同消费的最小语义；禁止仅输出不可执行的自由文本；当请求跨阶段收敛时，系统必须同步清理不再适用的历史状态字段，避免最终状态出现语义冲突。

#### 场景: 状态语义字段不完整

- **当** 一次策略执行完成后状态输出缺失任一语义字段
- **那么** 系统必须判定契约不通过并输出缺失项

#### 场景: 最终状态出现冲突语义

- **当** HealingRequest 已进入 completed 但仍保留旧的 blocked 字段、失败原因或影子动作
- **那么** 系统必须判定契约不通过并阻断交付

### 需求:HealingRequest 生成物必须可安装

系统必须为 HealingRequest 生成包含正确 group、version、resource name 的 CRD 生成物，禁止将无法直接安装到新集群的生成物视为通过的契约产出。

#### 场景: 新集群安装仓库内 CRD 生成物

- **当** 运维在一个全新的 Kubernetes 集群中直接安装仓库中的 HealingRequest CRD 生成物
- **那么** 系统必须成功创建 `healingrequests.kubesentinel.io` 资源，并暴露 `kubesentinel.io/v1alpha1` 版本

### 需求:HealingRequest 成功态必须清理过期阻断语义

系统必须在 HealingRequest 从 blocked 或 pending 阶段收敛为 completed 时清理过期的阻断原因、影子动作和失败语义，禁止让最终成功态同时表达旧的阻断结论。

#### 场景: 阻断后恢复并最终完成

- **当** 同一 HealingRequest 先经历门禁阻断或待验证阶段，随后在新的条件下执行成功并进入 completed
- **那么** 最终状态必须仅保留当前成功阶段所需的语义字段，不得残留旧的阻断原因或影子动作

### 需求:HealingRequest 必须暴露面向控制台和 Agent 的稳定状态语义

系统必须为 `HealingRequest` 暴露面向控制台和 Agent 共同消费的稳定状态语义，至少包括阶段、最近动作、失败或阻断原因、下一步建议和关联键；禁止让控制台或 Agent 只能依赖不可结构化的自由文本推断对象状态。

#### 场景: Agent 或控制台渲染对象摘要

- **当** Agent、Headlamp 或其他 Kubernetes 原生控制台渲染 `HealingRequest` 对象摘要
- **那么** 系统必须能够从稳定字段中直接读取阶段、最近动作、失败或阻断原因、下一步建议和关联键

### 需求:HealingRequest 必须提供适合列表视图的展示元数据

系统必须为 `HealingRequest` 提供适合通用 Kubernetes 控制台列表视图的展示元数据，例如打印列或等价 CRD 展示约束，使运维无需逐个展开对象详情即可识别关键运行态。

#### 场景: 控制台浏览 HealingRequest 列表

- **当** 运维在通用 Kubernetes 控制台中浏览 `HealingRequest` 列表
- **那么** 系统必须提供足以识别关键状态的列表展示元数据，而不要求用户逐个打开详情页才能完成基础筛查

### 需求:HealingRequest 必须暴露面向 Agent 的最小解释字段
系统必须为 Agent 暴露最小解释字段集合，至少包括阶段、最近动作、阻断或失败原因、下一步建议、关联键和目标 workload 标识，禁止要求 Agent 主要依赖不可结构化自由文本还原 incident。

#### 场景: Agent 读取 incident 状态
- **当** Agent 为某个 incident 生成摘要或建议
- **那么** 系统必须能够从稳定字段中直接读取阶段、动作、原因、建议和关联键
