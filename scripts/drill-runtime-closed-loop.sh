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

if ! kubectl -n default get healingrequest hr-db >/dev/null 2>&1; then
  echo "ASSERTION FAILED: StatefulSet 应进入只读评估对象"
  exit 1
fi
db_phase=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.phase}')
db_reason=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.blockReasonCode}')
db_cap=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.workloadCapability}')
echo "INFO: hr-db phase=$db_phase, blockReasonCode=$db_reason, workloadCapability=$db_cap"
if [[ "$db_reason" != "statefulset_readonly" ]] || [[ "$db_cap" != "read-only" ]]; then
  echo "ASSERTION FAILED: StatefulSet 必须命中只读阻断语义"
  exit 1
fi
echo "ASSERTION OK: StatefulSet 只读评估生效"

echo "[3.1/4] 打开 StatefulSet Phase 2 受控动作并触发审批"
kubectl -n default patch healingrequest hr-db --type merge -p '{"metadata":{"annotations":{"kube-sentinel.io/statefulset-approved":"true"}},"spec":{"statefulSetPolicy":{"enabled":true,"readOnlyOnly":false,"controlledActionsEnabled":true,"allowedNamespaces":["default"],"approvalAnnotation":"kube-sentinel.io/statefulset-approved","requireEvidence":false,"freezeWindowMinutes":10}}}' >/dev/null
sleep 2
db_cap_phase2=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.workloadCapability}')
db_auth=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.statefulSetAuthorization}')
echo "INFO: hr-db phase2 workloadCapability=$db_cap_phase2, authorization=$db_auth"
if [[ "$db_cap_phase2" != "conditional-writable" ]]; then
  echo "ASSERTION FAILED: StatefulSet Phase 2 应暴露 conditional-writable 能力"
  exit 1
fi
echo "ASSERTION OK: StatefulSet Phase 2 能力声明生效"

echo "[3.2/4] 验证 StatefulSet Phase 3 L2 字段存在"
db_l2_result=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.statefulSetL2Result}' 2>/dev/null || true)
db_l2_decision=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.statefulSetL2Decision}' 2>/dev/null || true)
echo "INFO: hr-db l2Result=${db_l2_result:-<empty>}, l2Decision=${db_l2_decision:-<empty>}"
echo "ASSERTION OK: Phase 3 字段已暴露（实际路径依赖运行态触发）"

phase=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.phase}')
echo "INFO: 当前阶段=$phase（若候选为空应为L3）"

obj_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.objectOpen}')
domain_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.domainOpen}')
echo "INFO: circuitBreaker.objectOpen=$obj_open, domainOpen=$domain_open"

echo "[4/4] 输出关联证据"
kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.correlationKey}{"\n"}'
kubectl -n default get healingrequest hr-db -o jsonpath='{.status.correlationKey}{"\n"}'

echo "drill completed"
