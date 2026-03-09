## ADDED Requirements

### 需求:首发 pilot 和 cutover 必须绑定 RC 版本

系统必须要求首发 pilot/cutover 仅针对已完成预生产验证的 RC 版本执行，禁止对未经过 RC 标识的工作区构建或未留痕版本直接执行首发 cutover。

#### 场景: 对 RC 版本执行首发 pilot

- **当** 团队推进首发版本进入 pilot/cutover 阶段
- **那么** 系统必须能够明确识别该版本对应的 RC 标识，并将 pilot/cutover 证据与该 RC 版本关联

#### 场景: 无 RC 标识尝试执行首发 cutover

- **当** 操作者试图对未完成 RC 发布或无法识别版本标识的构建执行首发 cutover
- **那么** 系统必须阻断该操作并要求先完成 RC 发布与证据绑定

### 需求:首发 pilot 观察结论必须显式说明范围边界

系统必须在首发 pilot/cutover 记录中显式说明当前观察结论仅覆盖 Deployment 自动闭环，禁止把 StatefulSet 自动写动作误记为首发已验证范围。

#### 场景: 生成首发 pilot 记录

- **当** 系统输出一次首发 pilot/cutover 记录
- **那么** 记录中必须说明首发覆盖的工作负载范围和排除项

#### 场景: 记录误宣称超出首发范围

- **当** pilot/cutover 记录把 StatefulSet 自动写动作标记为首发已验证范围
- **那么** 系统必须将该记录视为无效并要求更正范围声明
