## ADDED Requirements

### 需求:V1 pilot 分阶段切流状态机

系统必须使用固定阶段推进 V1 上线流程，阶段至少包括 `pilot_prepare`、`pilot_observe`、`cutover_ready`、`cutover_blocked`、`cutover_done`，禁止跨阶段跳转。

#### 场景: 顺序推进成功

- **当** 当前阶段满足前置条件且校验通过
- **那么** 系统必须仅允许推进到下一阶段并记录阶段切换证据

#### 场景: 非法跳转

- **当** 请求从 `pilot_prepare` 直接跳转到 `cutover_done`
- **那么** 系统必须阻断并输出 `invalid_stage_transition`

### 需求:cutover 决策包最小契约

系统必须输出机器可读 cutover decision pack，至少包含 `decision`、`failureCategory`、`pilotBatch`、`rollbackTarget`、`traceKey`、`approvalLevel`、`timestamp`。

#### 场景: 字段完整

- **当** 系统完成一次 pilot->cutover 判定
- **那么** 必须输出包含最小字段的决策包并可被脚本稳定解析

#### 场景: 字段缺失

- **当** 决策包缺少任一最小字段
- **那么** 系统必须输出 `block` 并禁止继续放量
