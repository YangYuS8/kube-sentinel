#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-default}"

echo "[1/4] 触发 Deployment 告警事件"
kubectl -n "$NAMESPACE" port-forward svc/kube-sentinel 8090:8090 >/tmp/kube-sentinel-pf.log 2>&1 &
PF_PID=$!
trap 'kill $PF_PID >/dev/null 2>&1 || true' EXIT

sleep 1
cat <<'JSON' | curl -sS -X POST http://127.0.0.1:8090/alertmanager/webhook -H 'content-type: application/json' -d @-
{
  "alerts": [
    {
      "status": "firing",
      "fingerprint": "drill-fp-001",
      "labels": {
        "workload_kind": "Deployment",
        "namespace": "default",
        "name": "demo-app"
      },
      "annotations": {
        "summary": "runtime drill"
      }
    }
  ]
}
JSON

echo "\n[2/4] 检查 HealingRequest 是否创建"
kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.metadata.name}{"\n"}'

echo "[3/4] 检查三项强断言（示例）"
cat <<'JSON' | curl -sS -X POST http://127.0.0.1:8090/alertmanager/webhook -H 'content-type: application/json' -d @-
{
  "alerts": [
    {
      "status": "firing",
      "fingerprint": "drill-fp-nondeploy",
      "labels": {
        "workload_kind": "StatefulSet",
        "namespace": "default",
        "name": "db"
      }
    }
  ]
}
JSON

if kubectl -n default get healingrequest hr-db >/dev/null 2>&1; then
  echo "ASSERTION FAILED: 非Deployment目标不应创建写操作对象"
  exit 1
fi
echo "ASSERTION OK: 仅Deployment写"

phase=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.phase}')
echo "INFO: 当前阶段=$phase（若候选为空应为L3）"

obj_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.objectOpen}')
domain_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.domainOpen}')
echo "INFO: circuitBreaker.objectOpen=$obj_open, domainOpen=$domain_open"

echo "[4/4] 输出关联证据"
kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.correlationKey}{"\n"}'

echo "drill completed"
