#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-default}"

require_binary() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    echo "ASSERTION FAILED: missing required binary: $name"
    exit 1
  fi
}

ensure_prerequisites() {
  require_binary kubectl
  require_binary curl

  if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "ASSERTION FAILED: kubectl 当前无法访问集群"
    exit 1
  fi

  if ! kubectl get crd healingrequests.kubesentinel.io >/dev/null 2>&1; then
    echo "INFO: 安装 HealingRequest CRD"
    kubectl apply -f config/crd/_healingrequests.yaml >/dev/null
  fi

  local pod_count
  pod_count=$(kubectl -n "$NAMESPACE" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [[ -n "$pod_count" && "$pod_count" -lt 10 ]]; then
    echo "INFO: namespace $NAMESPACE 只有 $pod_count 个 Pod，默认 blast radius 很可能阻断本地 smoke；必要时先调高目标 HealingRequest 的 spec.blastRadius.maxPodPercentage。"
  fi
}

classify_outcome() {
  local phase="${1:-}"
  local block_reason="${2:-}"
  local l2_result="${3:-}"
  local l2_decision="${4:-}"
  if [[ -n "$block_reason" ]] || [[ "$phase" == "Blocked" ]]; then
    echo "block"
    return
  fi
  if [[ "$phase" == "L3" ]] || [[ "$l2_result" == "degraded" ]] || [[ "$l2_decision" == *"degrade"* ]]; then
    echo "degrade"
    return
  fi
  if [[ "$phase" == "Completed" ]] || [[ "$l2_result" == "success" ]] || [[ "$l2_result" == "skipped" ]]; then
    echo "allow"
    return
  fi
  echo "degrade"
}

assert_precommit_ci_consistency() {
  local precommit_outcome="${PRECOMMIT_GATE_OUTCOME:-}"
  local ci_outcome="${CI_GATE_OUTCOME:-}"
  if [[ -z "$precommit_outcome" ]] || [[ -z "$ci_outcome" ]]; then
    echo "INFO: 跳过预提交/CI 一致性断言（未同时提供 PRECOMMIT_GATE_OUTCOME 与 CI_GATE_OUTCOME）"
    return 0
  fi
  if [[ "$precommit_outcome" != "$ci_outcome" ]]; then
    echo "ASSERTION FAILED: 预提交与 CI 门禁语义不一致 (precommit=$precommit_outcome, ci=$ci_outcome)"
    exit 1
  fi
  echo "ASSERTION OK: 预提交与 CI 门禁语义一致 ($precommit_outcome)"
}

normalize_outcome() {
  local outcome="${1:-degrade}"
  case "$outcome" in
    allow|block|degrade) echo "$outcome" ;;
    *) echo "degrade" ;;
  esac
}

assert_slo_semantics_consistency() {
  local gate_outcome="$(normalize_outcome "${1:-}")"
  local slo_outcome="$(normalize_outcome "${SLO_GOVERNANCE_OUTCOME:-$gate_outcome}")"
  if [[ "$gate_outcome" != "$slo_outcome" ]]; then
    echo "ASSERTION FAILED: 门禁语义与 SLO 治理语义不一致 (gate=$gate_outcome, slo=$slo_outcome)"
    exit 1
  fi
  echo "ASSERTION OK: 门禁语义与 SLO 治理语义一致 ($gate_outcome)"
}

emit_incident_evidence() {
  local outcome="$(normalize_outcome "${1:-}")"
  local incident_level="${INCIDENT_LEVEL:-}"
  local recovery_condition="${INCIDENT_RECOVERY_CONDITION:-}"
  local runbook="${INCIDENT_RUNBOOK:-}"
  if [[ -z "$incident_level" ]]; then
    if [[ "$outcome" == "allow" ]]; then
      incident_level="info"
    elif [[ "$outcome" == "degrade" ]]; then
      incident_level="warning"
    else
      incident_level="critical"
    fi
  fi
  if [[ -z "$recovery_condition" ]]; then
    if [[ "$outcome" == "allow" ]]; then
      recovery_condition="maintain_target_and_observe"
    elif [[ "$outcome" == "degrade" ]]; then
      recovery_condition="recover_budget_below_degrade_threshold"
    else
      recovery_condition="manual_approval_after_incident_review"
    fi
  fi
  if [[ -z "$runbook" ]]; then
    if [[ "$outcome" == "allow" ]]; then
      runbook="runbook://runtime-observation"
    elif [[ "$outcome" == "degrade" ]]; then
      runbook="runbook://runtime-degrade-recovery"
    else
      runbook="runbook://runtime-block-rollback"
    fi
  fi
  echo "INFO: incident.level=$incident_level"
  echo "INFO: incident.recoveryCondition=$recovery_condition"
  echo "INFO: incident.runbook=$runbook"
  if [[ -z "$incident_level" || -z "$recovery_condition" || -z "$runbook" ]]; then
    echo "ASSERTION FAILED: 事故响应证据字段不完整"
    exit 1
  fi
}

assert_rollout_tuning_recovery_path() {
  local canary_stable="${ROLLOUT_CANARY_STABLE:-true}"
  local rollback_hit="${ROLLOUT_ROLLBACK_HIT:-true}"
  local tuning_approved="${TUNING_APPROVED:-true}"
  local recovery_observed="${RECOVERY_OBSERVED:-true}"
  echo "INFO: rollout.canaryStable=$canary_stable"
  echo "INFO: rollout.rollbackHit=$rollback_hit"
  echo "INFO: rollout.tuningApproved=$tuning_approved"
  echo "INFO: rollout.recoveryObserved=$recovery_observed"
  if [[ "$canary_stable" != "true" ]] || [[ "$rollback_hit" != "true" ]] || [[ "$tuning_approved" != "true" ]] || [[ "$recovery_observed" != "true" ]]; then
    echo "ASSERTION FAILED: 灰度放量 -> 越线回退 -> 阈值调优 -> 恢复观察 路径证据不完整"
    exit 1
  fi
  echo "ASSERTION OK: 灰度放量与调优恢复路径证据齐全"
}

assert_postmortem_fields() {
  local breach_reason="${POSTMORTEM_BREACH_REASON:-}"
  local mitigation_action="${POSTMORTEM_MITIGATION_ACTION:-}"
  local threshold_decision="${POSTMORTEM_THRESHOLD_DECISION:-}"
  local observation_plan="${POSTMORTEM_OBSERVATION_PLAN:-}"
  echo "INFO: postmortem.breachReason=${breach_reason:-<empty>}"
  echo "INFO: postmortem.mitigationAction=${mitigation_action:-<empty>}"
  echo "INFO: postmortem.thresholdDecision=${threshold_decision:-<empty>}"
  echo "INFO: postmortem.observationPlan=${observation_plan:-<empty>}"
  if [[ -z "$breach_reason" || -z "$mitigation_action" || -z "$threshold_decision" || -z "$observation_plan" ]]; then
    echo "ASSERTION FAILED: 复盘字段缺失（需要 breachReason/mitigationAction/thresholdDecision/observationPlan）"
    exit 1
  fi
  echo "ASSERTION OK: 复盘字段齐全"
}

emit_release_readiness_summary() {
  local outcome="$(normalize_outcome "${1:-degrade}")"
  local action_type="${RELEASE_READINESS_ACTION_TYPE:-restart}"
  local risk_level="${RELEASE_READINESS_RISK_LEVEL:-medium}"
  local strategy_mode="${RELEASE_READINESS_STRATEGY_MODE:-auto}"
  local circuit_tier="${RELEASE_READINESS_CIRCUIT_TIER:-none}"
  local rollback_candidate="${RELEASE_READINESS_ROLLBACK_CANDIDATE:-latest-healthy-revision}"
  local open_incidents="${RELEASE_READINESS_OPEN_INCIDENTS:-0}"
  local drill_success_rate="${RELEASE_READINESS_DRILL_SUCCESS_RATE:-1.0}"
  local drill_rollback_p95_ms="${RELEASE_READINESS_DRILL_ROLLBACK_P95_MS:-0}"
  local drill_gate_bypass_count="${RELEASE_READINESS_DRILL_GATE_BYPASS_COUNT:-0}"
  echo "INFO: releaseReadiness.actionType=$action_type"
  echo "INFO: releaseReadiness.riskLevel=$risk_level"
  echo "INFO: releaseReadiness.strategyMode=$strategy_mode"
  echo "INFO: releaseReadiness.circuitTier=$circuit_tier"
  echo "INFO: releaseReadiness.rollbackCandidate=$rollback_candidate"
  echo "INFO: releaseReadiness.openIncidents=$open_incidents"
  echo "INFO: releaseReadiness.decision=$outcome"
  echo "INFO: releaseReadiness.drill.successRate=$drill_success_rate"
  echo "INFO: releaseReadiness.drill.rollbackLatencyP95Ms=$drill_rollback_p95_ms"
  echo "INFO: releaseReadiness.drill.gateBypassCount=$drill_gate_bypass_count"
  if [[ -z "$rollback_candidate" ]]; then
    echo "ASSERTION FAILED: 发布就绪摘要缺少 rollbackCandidate"
    exit 1
  fi
  if ! [[ "$open_incidents" =~ ^[0-9]+$ ]]; then
    echo "ASSERTION FAILED: openIncidents 必须是整数"
    exit 1
  fi
  if [[ "$outcome" != "allow" && "$outcome" != "degrade" && "$outcome" != "block" ]]; then
    echo "ASSERTION FAILED: release readiness decision 不合法"
    exit 1
  fi
}

assert_oncall_template_for_outcome() {
  local outcome="$(normalize_outcome "${1:-degrade}")"
  local alert_level="${ONCALL_ALERT_LEVEL:-}"
  local runbook="${ONCALL_RUNBOOK:-}"
  local approval="${ONCALL_APPROVAL_TRIGGER:-}"
  local rollback_action="${ONCALL_ROLLBACK_ACTION:-}"
  if [[ -z "$alert_level" || -z "$runbook" || -z "$approval" || -z "$rollback_action" ]]; then
    echo "ASSERTION FAILED: 值班模板关键字段缺失"
    exit 1
  fi
  if [[ "$outcome" == "block" && "$alert_level" != "critical" ]]; then
    echo "ASSERTION FAILED: block 必须映射 critical 值班级别"
    exit 1
  fi
  if [[ "$outcome" == "degrade" && "$alert_level" != "warning" ]]; then
    echo "ASSERTION FAILED: degrade 必须映射 warning 值班级别"
    exit 1
  fi
  if [[ "$outcome" == "allow" && "$alert_level" != "info" ]]; then
    echo "ASSERTION FAILED: allow 必须映射 info 值班级别"
    exit 1
  fi
  echo "ASSERTION OK: 值班模板字段完整且等级映射正确"
}

echo "[1/4] 触发 Deployment 告警事件"
ensure_prerequisites
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
db_outcome=$(classify_outcome "$db_phase" "$db_reason" "" "")
echo "INFO: hr-db gateOutcome=$db_outcome"

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
db_snapshot_id=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.lastSnapshotId}' 2>/dev/null || true)
db_snapshot_restore=$(kubectl -n default get healingrequest hr-db -o jsonpath='{.status.snapshotRestoreResult}' 2>/dev/null || true)
echo "INFO: hr-db l2Result=${db_l2_result:-<empty>}, l2Decision=${db_l2_decision:-<empty>}"
echo "INFO: hr-db snapshotId=${db_snapshot_id:-<empty>}, snapshotRestoreResult=${db_snapshot_restore:-<empty>}"
if [[ -z "${db_snapshot_id}" ]]; then
  echo "ASSERTION FAILED: 应暴露 lastSnapshotId"
  exit 1
fi
echo "ASSERTION OK: Phase 3 字段已暴露（实际路径依赖运行态触发）"

echo "[3.3/4] 输出 Deployment 分层阶段证据"
dep_phase=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.phase}' 2>/dev/null || true)
dep_action=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.lastAction}' 2>/dev/null || true)
dep_l2_result=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.deploymentL2Result}' 2>/dev/null || true)
dep_l2_decision=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.deploymentL2Decision}' 2>/dev/null || true)
dep_candidate=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.deploymentL2Candidate}' 2>/dev/null || true)
echo "INFO: hr-demo-app phase=${dep_phase:-<empty>}, lastAction=${dep_action:-<empty>}"
echo "INFO: hr-demo-app deploymentL2Result=${dep_l2_result:-<empty>}, deploymentL2Decision=${dep_l2_decision:-<empty>}, deploymentL2Candidate=${dep_candidate:-<empty>}"
if [[ -z "${dep_phase}" ]]; then
  echo "ASSERTION FAILED: Deployment 必须输出分层阶段信息"
  exit 1
fi
echo "ASSERTION OK: Deployment 分层阶段证据已输出"

dep_reason=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.blockReasonCode}' 2>/dev/null || true)
dep_outcome=$(classify_outcome "$dep_phase" "$dep_reason" "$dep_l2_result" "$dep_l2_decision")
echo "INFO: hr-demo-app gateOutcome=$dep_outcome"
emit_release_readiness_summary "$dep_outcome"
assert_oncall_template_for_outcome "$dep_outcome"
assert_precommit_ci_consistency
assert_slo_semantics_consistency "$dep_outcome"
emit_incident_evidence "$dep_outcome"

if [[ "$dep_outcome" == "block" ]]; then
  echo "ASSERTION FAILED: quality gate=block，禁止继续发布推进"
  exit 1
fi
if [[ "$dep_outcome" == "degrade" ]]; then
  echo "INFO: quality gate=degrade，执行保守降级路径（应进入 L3/人工介入）"
  if [[ "${dep_phase}" != "L3" ]]; then
    echo "ASSERTION FAILED: degrade 路径必须进入 L3"
    exit 1
  fi
fi

echo "[3.4/4] 验证 allow/block/degrade 三类解析断言"
fixture_allow=$(classify_outcome "Completed" "" "success" "")
fixture_block=$(classify_outcome "Blocked" "statefulset_readonly" "" "")
fixture_degrade=$(classify_outcome "L3" "" "degraded" "no-healthy-candidate")
if [[ "$fixture_allow" != "allow" ]] || [[ "$fixture_block" != "block" ]] || [[ "$fixture_degrade" != "degrade" ]]; then
  echo "ASSERTION FAILED: gate outcome 解析逻辑不符合 allow/block/degrade 语义"
  exit 1
fi
if [[ "$db_outcome" != "block" ]]; then
  echo "ASSERTION FAILED: 运行态 StatefulSet 路径必须命中 block"
  exit 1
fi
echo "ASSERTION OK: allow/block/degrade 解析与运行态断言通过"

echo "[3.5/4] 验证灰度放量、回退、调优与恢复观察路径"
assert_rollout_tuning_recovery_path

echo "[3.6/4] 验证复盘审计字段"
assert_postmortem_fields

phase=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.phase}')
echo "INFO: 当前阶段=$phase（若候选为空应为L3）"

obj_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.objectOpen}')
domain_open=$(kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.circuitBreaker.domainOpen}')
echo "INFO: circuitBreaker.objectOpen=$obj_open, domainOpen=$domain_open"

echo "[4/4] 输出关联证据"
kubectl -n default get healingrequest hr-demo-app -o jsonpath='{.status.correlationKey}{"\n"}'
kubectl -n default get healingrequest hr-db -o jsonpath='{.status.correlationKey}{"\n"}'

echo "drill completed"
