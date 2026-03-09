# V1 首发发布检查表

## 输入条件

- 目标版本已经明确区分为 RC 标签或稳定版标签。
- 当前发布范围只覆盖 Deployment 自动闭环；StatefulSet 自动写动作与额外 UI 查询层明确排除在首发阻断项之外。
- 统一质量门禁已通过：`make quality-gate`。
- 预生产验证入口可用：`install-minimal`、`dev-local-loop check`、`drill-runtime-closed-loop`、`make delivery-pipeline`。
- 首发证据归档目录已确定，建议使用 `.tmp/v1-release-execution/<stage>-<version>/`。

## RC 阶段检查

- [ ] 版本标签为预发布语义化版本，如 `v1.0.0-rc.1`。
- [ ] `release-image-plan` 输出 `channel=prerelease`。
- [ ] `release-image-plan` 输出 `publish_latest=false`。
- [ ] `drill-runtime-closed-loop` 覆盖默认阻断路径与单次放宽后的成功路径。
- [ ] `make delivery-pipeline` 输出 `DELIVERY_PIPELINE_RESULT=allow`。
- [ ] go-live decision pack 与 pilot/cutover decision pack 已准备好归档位置。

## 稳定版阶段检查

- [ ] 版本标签为稳定版语义化版本，如 `v1.0.0`。
- [ ] 已记录关联 RC 标签。
- [ ] 预生产验证已完成并留存证据。
- [ ] go-live 判定已完成并留存证据。
- [ ] pilot/cutover 已完成并留存证据。
- [ ] `release-image-plan` 输出 `channel=stable`。
- [ ] `release-image-plan` 输出 `publish_latest=true`。

## 预期行为

- 首发发布顺序固定为：预生产验证 -> RC 发布 -> pilot/cutover -> 稳定版发布 -> latest 更新。
- 发布记录必须显式声明首发范围只覆盖 Deployment 自动闭环。
- 证据包必须至少包含 decision pack、预生产演练输出、版本标签记录和回滚说明。
- 稳定版发布前必须具备人工交接所需字段与回滚基线。

## 失败路径

- 预发布标签缺失或使用稳定版标签执行 RC 阶段：阻断并回到 RC 规划。
- `publish_latest=true` 出现在 RC 阶段：阻断并修正发布计划。
- 缺少 go-live 或 pilot/cutover 证据时尝试发布稳定版：阻断并回到预生产/RC 阶段。
- 演练记录混入 StatefulSet 自动写动作作为首发阻断项：阻断并更正范围说明。
- 缺少 `handoffOwner`、`approvalLevel`、`traceKey`、`rollbackCommandRef` 或 `handoffTimestamp`：禁止稳定版放行。

## 人工交接要求

- 必填字段：`handoffOwner`、`approvalLevel`、`traceKey`、`rollbackCommandRef`、`handoffTimestamp`。
- 必须说明当前首发范围、排除项与回滚基线。
- 必须给出失败后的回退入口：关闭 Deployment L2、回到只读评估、根据快照与审计执行人工恢复。
