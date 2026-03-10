## 新增需求

### 需求:Telegram 必须成为多入口体验中的主动到达层
系统必须将 Telegram 明确为多入口体验中的主动到达层，使值班人员先在 Telegram 中收到 incident card，再进入 Agent、Headlamp、Grafana 或 kubectl 路径。

#### 场景: 值班人员收到主动通知
- **当** 某个 incident 触发主动通知
- **那么** 运维必须能够先从 Telegram 看到事件摘要，再继续进入其他入口排查

## 修改需求

### 需求:Telegram 必须作为 V1 唯一主动通知通道
系统必须将 Telegram 定义为 V1 唯一主动通知通道，并要求该通道具备真实发送闭环，而不是仅停留在模板或文案定义层。

#### 场景: 启用 Telegram 通知
- **当** 系统配置了 Telegram 发送能力并产生一个符合条件的 incident
- **那么** 系统必须实际向 Telegram 发送通知，而不是仅在本地生成消息模板

## 移除需求
