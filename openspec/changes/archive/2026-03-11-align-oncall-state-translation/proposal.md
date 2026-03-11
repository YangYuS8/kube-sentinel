## 为什么

当前 Kube-Sentinel 已经具备 runtime phase、Agent 五段式输出和 Telegram incident card，但真实本地演练表明：系统内部 phase 与夜间值班心智并不一致。像 `PendingVerify` 这样的运行时阶段会直接泄露到 drill、Agent 和 Telegram 语义里，导致值班人难以快速判断“系统正在观察、已阻断、已自动尝试还是已经恢复”。为了让这个轻量工具尽快稳定落地，需要引入一层明确的 oncall state 翻译，把真实 phase 收敛成值班语义。

## 变更内容

- 引入从 runtime phase 到 oncall state 的稳定翻译层。
- 将 `Pending` / `PendingVerify` 翻译为 `observing`，`Blocked` / `L3` 翻译为 `blocked`，`L1` / `Completed` 翻译为 `auto-tried`，`Suppressed` 翻译为 `recovered`。
- 让 Telegram 标题、Agent 运行时解释和 drill 验收默认基于 oncall state，而不是直接暴露 phase。
- 保留 phase 作为系统事实层，但不再让其主导值班表面。

## 功能 (Capabilities)

### 新增功能
- `oncall-state-translation`: 定义 runtime phase 到夜班值班语义的稳定翻译层。

### 修改功能
- `agent-v1-triage`: 让 Agent 输出区分事实层（phase）和值班层（oncall state）。
- `telegram-triage-delivery`: 让 Telegram 标题和通知类别基于 oncall state 生成。
- `runtime-closed-loop-validation`: 让 drill/验收脚本验证 oncall state，而不只是原始 phase。

## 影响

- 受影响的代码主要集中在 Agent 输出组装、Telegram 通知映射、drill/验收脚本和少量状态派生逻辑。
- 受影响的产品表面包括 Telegram 标题、Agent 文案、值班说明和本地演练断言。
- 该变更不涉及新增自动修复动作、多通道通知或大规模 legacy 清理。
