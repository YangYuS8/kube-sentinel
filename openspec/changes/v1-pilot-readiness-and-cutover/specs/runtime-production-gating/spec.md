## ADDED Requirements

### 需求:pilot 到 cutover 的单向门禁

生产门禁必须将 `pilot_observe` 通过作为 `cutover_ready` 的前置条件，禁止未完成观察窗口直接 cutover。

#### 场景: 观察窗口未完成

- **当** pilot 观察窗口未达到最小时长或关键指标未达标
- **那么** 系统必须阻断 cutover 并保持在 `pilot_observe`

#### 场景: 观察窗口完成并达标

- **当** pilot 观察窗口完成且关键指标达标
- **那么** 系统必须允许进入 `cutover_ready`

### 需求:cutover 失败自动回退

系统必须在 cutover 阶段触发高优先级失败条件时自动回退到上一稳定批次，并输出回退证据。

#### 场景: 触发高优先级失败

- **当** 出现熔断触发、关键 SLO 越线或证据不完整
- **那么** 系统必须自动回退并输出 `cutover_auto_rollback`

#### 场景: 未触发失败条件

- **当** cutover 期间未触发任何高优先级失败条件
- **那么** 系统必须完成 cutover 并写入 `cutover_done`
