# 主线A最小验收清单

## 输入条件
- Alertmanager Webhook 可访问：`/alertmanager/webhook`
- 目标 Deployment 存在且可识别
- HealingRequest CRD 已安装

## 预期行为
- 合法告警可创建/更新 `HealingRequest`
- 非 Deployment 事件仅只读拒绝，不触发写操作
- 无健康 Revision 时阶段进入 `L3`
- 熔断触发后自动写操作被阻断
- `status/event/metric/audit` 可通过 `correlationKey` 关联
- 自动写动作前必须生成 `status.lastSnapshotId`
- 回滚失败时必须记录 `status.snapshotRestoreResult`（`success` 或 `failed`）

## 失败路径
- 缺少关键 labels（`workload_kind/namespace/name`）返回可诊断错误
- 重复事件在幂等窗口内被抑制
- 门禁命中（维护窗口/速率限制/爆炸半径）时仅只读评估+告警
- 证据不足时禁止 L2 回滚并输出人工介入建议
- 快照创建失败时必须阻断写操作并输出 `snapshot-failed`
- 快照恢复失败时必须进入冻结并输出人工介入建议
