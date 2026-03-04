## 为什么

当前 Kube-Sentinel 的核心运行时能力已基本具备，但交付稳定性仍依赖“人工记忆 + 临时执行”。最近 CI 连续失败暴露了同一类问题：API/CRD/Chart 产物不同步、检查口径不一致、失败诊断不够直接。为了持续按敏捷节奏迭代并快速上线，需要把质量门禁从“建议动作”升级为“默认执行且可诊断”的交付基线。

## 变更内容

- 建立统一质量门禁入口：将 `go test ./...`、`go test -race`（核心路径）、`go vet ./...`、`golangci-lint run`、CRD 一致性检查收敛为可重复执行的标准流程。
- 固化 CRD/API 一致性策略：约束 API 字段变更必须伴随 CRD 生成物同步，避免在 CI 末端才暴露漂移。
- 提升失败可诊断性：统一输出“失败类型 -> 根因提示 -> 修复建议”的可读报告，降低排障往返成本。
- 强化交付门禁策略：把质量门禁前移为默认阻断条件，明确允许跳过的例外场景与审批语义（仅限必要维护场景）。
- 引入迭代级验收基线：将“预提交检查 + CI 检查 + 演练脚本检查”映射到同一验收矩阵，保证每次迭代可验证。

## 功能 (Capabilities)

### 新增功能
- `delivery-quality-gates`: 定义交付质量门禁的统一执行入口、失败分类与最小阻断策略。

### 修改功能
- `runtime-production-hardening`: 增加“交付门禁即发布门禁”的约束语义，补齐质量检查阻断与恢复条件。
- `runtime-closed-loop-validation`: 扩展闭环验收矩阵，纳入质量门禁通过性与失败路径断言。

## 影响

- 代码与脚本：`scripts/check-crd-consistency.sh`、`scripts/drill-runtime-closed-loop.sh`、`Makefile`、CI workflow（如 `.github/workflows/*`）。
- 工程质量：`golangci-lint`、`go test -race` 的执行路径和耗时策略需明确分层（全量/核心路径）。
- API/CRD：涉及 `api/v1alpha1` 变更时需同步 `config/crd` 产物并通过一致性校验。
- 交付流程：PR 合并前需要显式满足质量门禁，不再依赖“合并后补修”。
