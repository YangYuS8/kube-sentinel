## ADDED Requirements

### 需求:pilot 期间 SLO 退化触发矩阵

系统必须在 pilot 期间按阈值将 SLO 状态映射为 `observe_only`、`pause_rollout`、`rollback_required` 三类动作。

#### 场景: 中度退化

- **当** SLO 进入退化区间但未达到阻断阈值
- **那么** 系统必须执行 `pause_rollout` 并保持当前批次

#### 场景: 严重退化

- **当** SLO 达到阻断阈值或持续退化超过窗口阈值
- **那么** 系统必须执行 `rollback_required` 并触发回退流程

### 需求:回退阈值边界一致性

系统必须保证回退阈值边界在质量门禁、运行时门禁与决策包中语义一致。

#### 场景: 阈值一致

- **当** 三处组件读取同一阈值配置
- **那么** 系统必须输出一致的动作判定

#### 场景: 阈值不一致

- **当** 任意组件阈值解释不一致
- **那么** 系统必须阻断放量并输出 `slo_threshold_contract_mismatch`
