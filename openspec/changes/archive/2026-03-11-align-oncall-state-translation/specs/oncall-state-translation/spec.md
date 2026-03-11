## 新增需求

### 需求:系统必须提供 oncall state 翻译层
系统必须能够从 runtime phase 稳定推导出夜班值班语义状态，至少覆盖 `observing`、`blocked`、`auto-tried` 和 `recovered`，禁止要求值班人员直接理解所有内部 phase 才能判断当前状态。

#### 场景: 从 PendingVerify 推导值班状态
- **当** runtime phase 为 `PendingVerify`
- **那么** 系统必须将其翻译为 `observing`

#### 场景: 从 Completed 推导值班状态
- **当** runtime phase 为 `Completed`
- **那么** 系统必须将其翻译为 `auto-tried`

### 需求:多个 runtime phase 允许映射到同一个 oncall state
系统必须允许多个内部 phase 映射为同一个 oncall state，以收缩值班心智并避免暴露实现细节。

#### 场景: Blocked 和 L3 映射一致
- **当** runtime phase 为 `Blocked` 或 `L3`
- **那么** 系统必须将它们统一翻译为 `blocked`

## 修改需求

## 移除需求
