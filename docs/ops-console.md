# Headlamp + Grafana 运维控制台接入

本文档描述 Kube-Sentinel 首版运维控制台的最小接入方式。范围明确分为两类：

- 对象视图：通过 Headlamp 或其他 Kubernetes 原生控制台直接查看 HealingRequest、关联 workload 和 K8s Event。
- 指标视图：通过 Prometheus + Grafana 直接消费 kube*sentinel*\* 指标查看趋势。

当前不提供自定义 UI 后端，也不提供新的写操作入口。控制台集成仅用于只读诊断与观测。

## 前置条件

对象视图前置条件：

- 集群中已安装 Kube-Sentinel，且 HealingRequest CRD 已注册。
- 使用者可通过 kubeconfig、Headlamp Desktop 或集群内 Headlamp 访问目标集群。
- 集群中已有 HealingRequest 对象，或已执行过至少一次 webhook / drill 产生对象。

指标视图前置条件：

- 集群中已安装 Kube-Sentinel，并暴露 kube-sentinel-metrics Service。
- Prometheus 能抓取 kube-sentinel-metrics Service。若使用 Prometheus Operator，可直接复用 [config/monitoring/kube-sentinel-servicemonitor.yaml](../config/monitoring/kube-sentinel-servicemonitor.yaml)。
- Grafana 已配置 Prometheus 数据源，并可导入 [config/monitoring/kube-sentinel-grafana-dashboard.json](../config/monitoring/kube-sentinel-grafana-dashboard.json)。

## 对象视图验证

1. 安装或连接 Headlamp 到目标集群。
2. 打开 HealingRequest 资源列表。
3. 确认列表列中直接可见以下字段：
   - Phase
   - Action
   - Reason
   - Recommendation
   - Correlation
4. 打开任一对象详情，确认 status 中至少包含：phase、lastAction、blockReasonCode 或 lastError、nextRecommendation、correlationKey。
5. 如需关联排障，继续查看同 namespace 下的 workload 和相关 K8s Event。

对象视图重点是单个请求的当前状态与关联键，不负责展示聚合趋势。

## 指标视图验证

先确认标准抓取入口存在：

```bash
kubectl -n kube-sentinel-system get svc kube-sentinel-metrics
kubectl -n kube-sentinel-system get endpoints kube-sentinel-metrics
```

如果测试环境使用 Prometheus Operator，可应用最小 ServiceMonitor 资产：

```bash
kubectl apply -f config/monitoring/kube-sentinel-servicemonitor.yaml
```

然后在 Grafana 中导入最小 dashboard：

```text
config/monitoring/kube-sentinel-grafana-dashboard.json
```

导入后至少验证四组面板：

- 总体触发与成功率
- 策略执行结果
- 快照结果
- 门禁与发布趋势

如果面板为空，优先检查：

- Prometheus 是否已发现 kube-sentinel-metrics Service
- ServiceMonitor namespace 是否与安装 namespace 一致
- Grafana 数据源是否指向对应 Prometheus
- 测试窗口内是否确实产生了 kube*sentinel*\* 指标样本

指标视图重点是趋势和分组，不替代对象详情。定位单个请求时，应先从 HealingRequest 的 correlationKey、namespace 和 workload 标识入手，再切换到对应趋势面板继续排查。

## 本地开发回路中的分工

对象视图验证：

- 先执行 [scripts/install-minimal.sh](../scripts/install-minimal.sh) 或 [scripts/dev-local-loop.sh](../scripts/dev-local-loop.sh) 完成 CRD 与控制器安装。
- 通过 Headlamp 或 kubectl 查看 HealingRequest 列表与详情，确认打印列和 status 语义可读。

指标视图验证：

- 在相同集群中确认 kube-sentinel-metrics Service 存在。
- 为 Prometheus 配置抓取，再把 dashboard JSON 导入 Grafana。
- 运行一次 `bash ./scripts/drill-runtime-closed-loop.sh default` 生成指标样本，再观察面板变化。

不要把控制台验证和控制器功能验证混为一步：

- 控制器功能验证关注闭环是否执行正确。
- 对象视图验证关注资源和状态语义是否便于排障。
- 指标视图验证关注 Prometheus 抓取和面板分组是否稳定。
