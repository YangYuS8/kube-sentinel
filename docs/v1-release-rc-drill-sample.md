# V1 首发 RC 发布预演记录（样例）

## 演练输入

- 执行时间：2026-03-09T10:30:00Z
- 演练阶段：RC
- 目标版本：v1.0.0-rc.1
- 范围声明：deployment-only
- 排除项：StatefulSet 自动写动作、额外 UI 查询层

## 执行方式

本次预演通过统一入口执行：

```bash
V1_RELEASE_STAGE=rc \
V1_RELEASE_VERSION_TAG=v1.0.0-rc.1 \
V1_RELEASE_INSTALL_CMD="printf 'install ok\n'" \
V1_RELEASE_DEV_CHECK_CMD="printf 'dev check ok\n'" \
V1_RELEASE_SMOKE_CMD="printf 'smoke ok\n'" \
V1_RELEASE_PIPELINE_CMD="make delivery-pipeline" \
bash ./scripts/v1-release-execution.sh
```

说明：本次样例使用模拟的 install/dev/smoke 前置，重点验证首发顺序、delivery pipeline 放行结果与 RC 版本语义输出的串联完整性。

## 产出（摘录）

- `V1_RELEASE_SEQUENCE_RESULT=pass`
- `V1_RELEASE_STAGE=rc`
- `V1_RELEASE_VERSION_TAG=v1.0.0-rc.1`
- `V1_RELEASE_PLAN_CHANNEL=prerelease`
- `V1_RELEASE_PLAN_PUBLISH_LATEST=false`
- `DELIVERY_PIPELINE_RESULT=allow`

## 归档位置（样例）

- 归档目录：`.tmp/v1-release-execution/rc-v1.0.0-rc.1/`
- 发布摘要：`release-summary.env`
- 版本标签记录：`release-plan.env`
- 交付流水线输出：`04-delivery-pipeline.log`

已固化的样例证据：

- [docs/evidence/v1-release-rc-summary.sample.env](docs/evidence/v1-release-rc-summary.sample.env)
- [docs/evidence/v1-release-rc-plan.sample.env](docs/evidence/v1-release-rc-plan.sample.env)

## 复盘结论

- 当前首发预演路径可以在不扩展核心功能的前提下完成 RC 阶段串联。
- 首发阻断范围保持为 Deployment 自动闭环，没有将 StatefulSet 自动写动作误纳入放行前置。
- 稳定版放行前仍需补齐 go-live、pilot/cutover 和人工交接证据。
