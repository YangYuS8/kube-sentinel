## 为什么

当前版本已经具备 Deployment L1 的首发闭环与质量门禁，但部署与本地验证入口仍偏开发者手工流程：需要分别安装 CRD、启动 manager、准备测试对象、处理 minikube 代理与 blast radius 细节。继续直接扩展 L2/L3 或 StatefulSet 能力，会让后续验证成本持续升高，因此现在需要先把“可安装、可启动、可复现”的本地开发回路收敛为稳定入口。

## 变更内容

- 提供一个面向本地与测试环境的最小部署打包路径，使 Kube-Sentinel 不再只依赖 `go run ./cmd/manager` 的手工启动方式。
- 收敛本地开发回路，统一 CRD 安装、manager 启动、示例 workload、smoke 验证与环境自检步骤。
- 固化 minikube 友好的运行约束，降低代理配置、旧进程残留、低 Pod 基数导致 blast radius 阻断等常见误判成本。
- 补齐与上述入口对应的文档、脚本和验收标准，使后续功能切片能够在同一条本地/灰度路径上重复验证。

## 功能 (Capabilities)

### 新增功能

- `local-deployment-and-dev-loop`: 定义 Kube-Sentinel 的本地部署打包入口、开发者自检流程与最小 smoke 验证回路。

### 修改功能

- `runtime-closed-loop-validation`: 将本地 smoke 的前置条件、阻断/成功双路径与可重复验证要求收敛到统一的开发回路中。

## 影响

- 受影响代码：部署脚本、开发脚本、README、联调文档、Helm/chart 安装入口与 smoke 验证脚本。
- 受影响系统：本地 minikube 环境、测试环境中的 CRD/manager 安装流程、开发者日常调试路径。
- 依赖与接口：可能新增或调整本地部署命令入口、Helm chart 模板与 demo workload 约定，但不改变现有 HealingRequest API 语义。
