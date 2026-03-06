## 上下文

首发 Deployment L1 MVP 已通过单元测试与本地质量门禁，但最近一次真实 minikube 闭环联调暴露出三类运行时缺口。第一，仓库中的 HealingRequest CRD 生成物无法直接安装到新集群，说明 API 包的 controller-gen 元数据链路不完整。第二，HealingRequest 在 PendingVerify 阶段依赖频繁状态写回推进时间窗口，在对象被人工 patch 或连续重入时容易触发 resourceVersion 冲突。第三，请求从 blocked 或 pending 进入 completed 后，旧的阻断字段没有被系统性清理，导致最终状态混合了历史阶段的语义。

这三个问题分别落在安装面、控制面和状态语义面，但它们在联调中是串联出现的：CRD 不可安装会阻断集群启动；状态冲突会让联调路径不稳定；状态残留会让成功结果不可诊断。修复必须一次性覆盖这三条链路，否则仍然会留下隐患。

## 目标 / 非目标

**目标：**

- 让仓库中的 HealingRequest CRD 生成物可直接安装到全新集群，并在生成检查中验证正确性而非仅验证目录一致性。
- 让 HealingRequest 的 PendingVerify 与后续状态推进改为冲突可控的 reconcile 模式，避免持续的对象版本冲突。
- 让 Deployment 首发 L1 在 blocked、pending、completed 之间切换时输出收敛的最终状态，不残留过期的阻断字段。
- 为真实集群闭环验证补齐最小 smoke 基线和文档说明。

**非目标：**

- 不引入新的 API 版本，也不扩大首发版本的工作负载范围。
- 不改变 Deployment L1 之外的首发能力边界，不重新开启 Deployment L2/L3 自动升级。
- 不在本次变更中完整重构 StatefulSet 多阶段流程，除非它与共享的状态持久化逻辑直接相关。

## 决策

### 决策 1: 通过包级 kubebuilder 元数据修复 CRD 生成源，而不是手工维护生成物

- 选择：补齐 `api/v1alpha1` 包级 controller-gen 元数据，让 `controller-gen` 直接生成正确的 group、version、resource name。
- 原因：当前问题的根因在生成源而不在产物目录，手工修补 `config/crd` 只能暂时掩盖问题，下一次生成仍会漂移。
- 备选方案：
  - 手工修正 `config/crd/_healingrequests.yaml`：短期可用，但无法通过生成一致性约束。
  - 放弃生成，改为人工维护 CRD：违背现有质量门禁与声明式 API 设计原则。

### 决策 2: PendingVerify 使用显式重试窗口推进，状态持久化使用冲突友好的 patch/retry 模式

- 选择：将 soak 窗口推进改为 `RequeueAfter` 驱动，减少为“推进时间”而重复写 status；controller 侧改用基于最新对象的状态 patch 或冲突重试策略。
- 原因：当前 `Status().Update` + 时间窗口状态自推进会形成自激式 reconcile，人工 patch 或高频重入时必然增加冲突概率。
- 备选方案：
  - 保持现有 `Status().Update` 并在报错后忽略：会吞掉状态推进失败，留下不可预测的最终语义。
  - 完全交给 orchestrator 内部做持久化：会进一步混淆 controller 与领域逻辑的职责边界。

### 决策 3: 为 HealingRequest 增加阶段切换归一化，而不是仅在空字段上补默认值

- 选择：在进入 blocked、pending、completed 等终态或准终态前，显式清理不再适用的历史字段，再补充当前阶段所需语义。
- 原因：`ensureStatusSemantics` 当前只负责“补空”，不负责“去旧”，因此无法保证最终状态只代表当前阶段。
- 备选方案：
  - 在每条成功路径上零散清理旧字段：容易遗漏，并在未来新增阶段时继续扩散。
  - 仅靠审计与日志表达真实结果：无法满足 CRD 状态语义的可检索要求。

### 决策 4: 把联调 smoke 验证纳入交付基线，但保持最小规模

- 选择：增加一个最小真实集群 smoke 路径，至少覆盖 CRD 安装、manager 启动、Webhook 接入、HealingRequest 创建、门禁/执行结果观测。
- 原因：这次问题都无法被纯单测暴露，必须有一条真实集群路径兜底。
- 备选方案：
  - 只补 README：无法形成可回归的验证基线。
  - 引入完整 e2e 框架：当前范围过大，不适合作为本次最小修复的前置条件。

## 风险 / 权衡

- [状态推进改为 RequeueAfter 后测试基线变化] → 需要更新 PendingVerify 相关单测，明确“时间推进靠重试而非频繁写 status”。
- [状态归一化可能影响现有依赖旧字段的断言] → 先补充成功态/阻断态收敛测试，再修改实现，避免误删仍需保留的诊断字段。
- [CRD 正确性校验变严后会暴露更多历史问题] → 将“正确且可安装”纳入脚本和测试输出，避免以后继续把坏产物同步进仓库。
- [真实集群 smoke 依赖本地网络与代理环境] → 文档中明确代理和 blast radius 对联调结果的影响，降低误诊概率。

## Migration Plan

1. 为 API 包补齐 generator 元数据并重新生成 CRD。
2. 加强 CRD 一致性与正确性测试，确保新生成物可直接安装。
3. 调整 controller/orchestrator 的状态推进与 patch 策略，并补充冲突场景测试。
4. 增加状态归一化测试，覆盖 blocked → completed、pending → completed 等转换。
5. 在本地集群上执行最小 smoke 验证，并同步更新 README 与联调说明。

回滚策略：

- 如果状态推进改动引发不可接受的回归，可回退到变更前的 controller/orchestrator 逻辑，但必须保留 CRD 生成修复与文档修正。
- 如果 CRD schema 强化导致安装问题，可暂时保留宽松 schema，但不能回退到空 group/version/name 的错误生成物。

## Open Questions

- 本次是否需要把 smoke 验证固化为 CI 中的可选目标，还是先作为本地交付基线保留。
- 状态归一化是否应进一步抽象为共享的阶段转换助手，以便后续 StatefulSet 路径复用。
