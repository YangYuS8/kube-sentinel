## 1. 首发范围与发布文档收敛

- [x] 1.1 更新首发版本说明与发布文档，明确 Deployment 自动闭环是首发范围，StatefulSet 自动写动作与额外 UI 查询层不属于首发阻断项。
- [x] 1.2 在发布与回滚说明中补齐统一首发顺序：预生产验证 -> RC 发布 -> pilot/cutover -> 稳定版发布 -> latest 更新，并写明失败路径与回滚入口。
- [x] 1.3 为首发证据包补充固定清单与归档位置说明，至少覆盖 go-live decision pack、pilot/cutover decision pack、预生产演练输出、版本标签记录和回滚说明。

## 2. 发布链路与自动化入口收敛

- [x] 2.1 调整现有发布脚本或 workflow，使 RC 与稳定版标签推进顺序满足首发规格，并为相关脚本补充对应单元测试。
- [x] 2.2 收敛预生产演练入口，确保 install-minimal、dev-local-loop、drill-runtime-closed-loop 与 delivery-pipeline 可以组成单一路径，并为关键失败路径补充可验证测试。
- [x] 2.3 校验 release-image、delivery pipeline 与版本语义输出的一致性，确保 latest 仅在稳定版发布时更新，并补齐回归测试。

## 3. 首发证据与发布预演

- [x] 3.1 生成一次首发预生产演练记录模板或正式样例，明确首发阻断范围、排除项与回滚基线。
- [x] 3.2 以 RC 版本为目标执行一次首发发布预演，验证 go-live 判定、pilot/cutover 记录、证据归档和版本标识串联完整。
- [x] 3.3 基于预演结果更新最终发布检查表，明确稳定版发布前的输入条件、预期行为、失败路径与人工交接要求。
