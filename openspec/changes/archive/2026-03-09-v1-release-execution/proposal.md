## 为什么

Kube-Sentinel 的首发范围、质量门禁、镜像发布和 go-live 规格已经分别具备，但它们仍分散在多个脚本、样例和规格中，缺少一个面向首个可交付版本的统一发布执行闭环。现在需要把现有 Deployment 闭环能力收束成可重复执行的 v1 发布路径，确保首发版本可以被证明“可验证、可交付、可回滚”，而不是继续扩展尚不属于首发边界的功能。

## 变更内容

- 定义 v1 首发发布执行范围，明确 Deployment 自动闭环是首发交付对象，StatefulSet 自动写动作继续排除在首发验收之外。
- 将预生产实跑、go-live 判定、pilot/cutover、镜像标签发布和证据归档串联为单一发布执行流程。
- 固化 RC 到稳定版的版本语义，要求先完成预发布验证，再推进稳定版与 latest 发布。
- 明确首发所需的最小证据包、发布记录和回滚说明，避免发布判断依赖口头确认或零散文档。

## 功能 (Capabilities)

### 新增功能

- `v1-release-execution`: 定义首个可交付版本从预生产演练、RC 发布、pilot/cutover 到稳定版发布的统一执行闭环与最小证据要求。

### 修改功能

- `v1-go-live-readiness`: 将 go-live 判定从通用闸门能力收束到首发发布执行语境，补充首发必需证据与发布顺序要求。
- `v1-pilot-readiness-and-cutover`: 明确首发 pilot/cutover 在 Deployment 首发场景下的推进前置、观察窗口和回退要求。
- `container-image-distribution`: 明确首发阶段 RC 标签与稳定版标签的推进顺序，以及 latest 仅在稳定版发布时更新。
- `local-deployment-and-dev-loop`: 补充首发预生产演练入口与发布执行流程之间的衔接要求，确保 install/dev-loop/smoke 可作为发布前验证基线。

## 影响

- OpenSpec 规格：新增首发发布执行能力规范，并修改 go-live、pilot/cutover、镜像分发、本地联调相关规范。
- 发布文档与运行手册：需要统一首发范围、预生产演练、RC 发布、稳定版发布和回滚步骤。
- 发布脚本与工作流：可能需要调整现有 delivery pipeline、release-image workflow 与演练入口的衔接方式，但不扩展新的核心自愈功能。
