## 1. 修复 CRD 生成链路

- [ ] 1.1 为 `api/v1alpha1` 补齐 controller-gen 所需的包级元数据并重新生成 HealingRequest CRD（验收：`config/crd/_healingrequests.yaml` 包含正确的 `healingrequests.kubesentinel.io`、`kubesentinel.io`、`v1alpha1`，且可直接 `kubectl apply` 到新集群）。
- [ ] 1.2 加强 CRD 一致性与正确性测试，覆盖“目录一致但生成物错误”的失败路径（验收：错误的 group/version/name 或不可安装清单会阻断脚本或测试；包含对应单元测试方案）。

## 2. 收敛 Reconcile 冲突

- [ ] 2.1 重构 HealingRequest controller 的状态持久化路径，改为冲突可重试的 status patch 或等价策略（验收：对象被并发 patch 时不再持续出现 `object has been modified`；包含冲突重试单元测试）。
- [ ] 2.2 调整 PendingVerify/soak 窗口推进方式，使用显式重试窗口而不是高频状态自更新（验收：待验证请求可继续推进到下一阶段，且失败路径覆盖人工 patch、控制器重入和幂等性场景）。

## 3. 归一化最终状态语义

- [ ] 3.1 为 HealingRequest 增加阶段切换归一化逻辑，统一清理 blocked、pending、completed 之间不再适用的历史字段（验收：blocked → completed、pending → completed、failed → completed 不再残留旧的 `blockReasonCode`、`shadowAction`、失败原因；包含单元测试）。
- [ ] 3.2 对齐审计、事件和状态语义输出，使最终成功态仅反映本次成功执行的门禁裁决、动作和建议（验收：相关测试与联调结果中不再出现成功态混杂旧阻断语义）。

## 4. 补齐交付验证与文档

- [ ] 4.1 增加最小真实集群 smoke 验证步骤或脚本，覆盖 CRD 安装、manager 启动、Webhook 接入、HealingRequest 创建与 Deployment L1 执行/阻断结果（验收：本地 minikube 可重复复现；包含失败路径说明）。
- [ ] 4.2 更新 README 与联调说明，明确代理前置条件、CRD 安装顺序，以及小规模集群下 blast radius 对首发闭环验证的影响（验收：文档可指导新环境完成同一轮闭环联调）。

## 5. 完成质量门禁

- [ ] 5.1 运行并通过与本变更相关的测试、CRD 一致性检查和最小闭环验证（验收：`go test ./...`、相关 race/vet/lint/CRD 检查通过，且记录本次 smoke 验证结果）。
