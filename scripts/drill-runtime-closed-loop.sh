#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-default}"
SYSTEM_NAMESPACE="${KUBE_SENTINEL_SYSTEM_NAMESPACE:-kube-sentinel-system}"
MANAGER_MODE="${KUBE_SENTINEL_MANAGER_MODE:-local}"
WEBHOOK_URL="${KUBE_SENTINEL_WEBHOOK_URL:-http://127.0.0.1:8090/alertmanager/webhook}"
DEMO_NAME="${KUBE_SENTINEL_DEMO_NAME:-kube-sentinel-smoke-$(date +%s)}"
BLOCK_TIMEOUT_SECONDS="${KUBE_SENTINEL_SMOKE_BLOCK_TIMEOUT_SECONDS:-60}"
SUCCESS_TIMEOUT_SECONDS="${KUBE_SENTINEL_SMOKE_SUCCESS_TIMEOUT_SECONDS:-90}"
RELAXED_MAX_POD_PERCENT="${KUBE_SENTINEL_SMOKE_RELAXED_MAX_POD_PERCENT:-100}"
SOAK_DURATION_SECONDS="${KUBE_SENTINEL_SMOKE_SOAK_DURATION_SECONDS:-1}"
SOAK_MIN_SAMPLES="${KUBE_SENTINEL_SMOKE_SOAK_MIN_SAMPLES:-1}"
KEEP_SMOKE_RESOURCES="${KUBE_SENTINEL_KEEP_SMOKE_RESOURCES:-false}"
POLL_INTERVAL_SECONDS=2
PF_PID=""
CREATED_DEMO="false"

fail() {
  echo "ASSERTION FAILED: $*"
  exit 1
}

info() {
  echo "INFO: $*"
}

require_binary() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    fail "missing required binary: $name"
  fi
}

cleanup() {
  if [[ -n "$PF_PID" ]]; then
    kill "$PF_PID" >/dev/null 2>&1 || true
  fi
  if [[ "$KEEP_SMOKE_RESOURCES" != "true" ]]; then
    kubectl -n "$NAMESPACE" delete healingrequest "hr-${DEMO_NAME}" --ignore-not-found >/dev/null 2>&1 || true
    if [[ "$CREATED_DEMO" == "true" ]]; then
      kubectl -n "$NAMESPACE" delete service "$DEMO_NAME" --ignore-not-found >/dev/null 2>&1 || true
      kubectl -n "$NAMESPACE" delete deployment "$DEMO_NAME" --ignore-not-found >/dev/null 2>&1 || true
    fi
  fi
}

trap cleanup EXIT

ensure_prerequisites() {
  require_binary kubectl
  require_binary curl

  if ! kubectl cluster-info >/dev/null 2>&1; then
    fail "kubectl 当前无法访问集群"
  fi

  if ! kubectl get crd healingrequests.kubesentinel.io >/dev/null 2>&1; then
    info "installing HealingRequest CRD"
    kubectl apply -f config/crd/_healingrequests.yaml >/dev/null
  fi

  ensure_demo_workload

  local pod_count
  pod_count=$(kubectl -n "$NAMESPACE" get pods --no-headers 2>/dev/null | wc -l | tr -d ' ')
  if [[ -n "$pod_count" && "$pod_count" -lt 10 ]]; then
    info "namespace ${NAMESPACE} 只有 ${pod_count} 个 Pod，默认 blast radius 会先阻断 smoke；后续脚本会仅对当前 HealingRequest 临时放宽，不应复用到生产默认值。"
  fi

  if [[ "$MANAGER_MODE" == "cluster" ]]; then
    kubectl -n "$SYSTEM_NAMESPACE" get svc kube-sentinel >/dev/null 2>&1 || fail "missing service ${SYSTEM_NAMESPACE}/kube-sentinel"
    kubectl -n "$SYSTEM_NAMESPACE" port-forward svc/kube-sentinel 8090:8090 >/tmp/kube-sentinel-pf.log 2>&1 &
    PF_PID=$!
    sleep 1
  fi
}

ensure_demo_workload() {
  if kubectl -n "$NAMESPACE" get deployment "$DEMO_NAME" >/dev/null 2>&1; then
    info "reusing smoke deployment ${NAMESPACE}/${DEMO_NAME}"
    return
  fi
  CREATED_DEMO="true"
  info "creating temporary smoke deployment ${NAMESPACE}/${DEMO_NAME}"
  cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${DEMO_NAME}
  namespace: ${NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${DEMO_NAME}
  template:
    metadata:
      labels:
        app: ${DEMO_NAME}
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: ${DEMO_NAME}
  namespace: ${NAMESPACE}
spec:
  selector:
    app: ${DEMO_NAME}
  ports:
    - port: 80
      targetPort: 80
EOF
}

wait_for_condition() {
  local description="$1"
  local timeout_seconds="$2"
  shift 2
  local deadline=$((SECONDS + timeout_seconds))
  while (( SECONDS < deadline )); do
    if "$@"; then
      return 0
    fi
    sleep "$POLL_INTERVAL_SECONDS"
  done
  fail "timeout waiting for ${description}"
}

jsonpath_value() {
  local resource="$1"
  local path="$2"
  kubectl -n "$NAMESPACE" get "$resource" -o "jsonpath=${path}" 2>/dev/null || true
}

oncall_state_from_phase() {
  local phase="$1"
  case "$phase" in
    Pending|PendingVerify)
      printf '%s' "observing"
      ;;
    Blocked|L3)
      printf '%s' "blocked"
      ;;
    L1|Completed)
      printf '%s' "auto-tried"
      ;;
    Suppressed)
      printf '%s' "recovered"
      ;;
    *)
      printf '%s' "blocked"
      ;;
  esac
}

cleanup_previous_state() {
  info "cleaning previous HealingRequest state"
  kubectl -n "$NAMESPACE" delete healingrequest "hr-${DEMO_NAME}" --ignore-not-found >/dev/null
  wait_for_condition "hr-${DEMO_NAME} deletion" 20 bash -lc "[[ -z \"\$(kubectl -n '${NAMESPACE}' get healingrequest 'hr-${DEMO_NAME}' --ignore-not-found -o name 2>/dev/null)\" ]]"
}

send_demo_alert() {
  local fingerprint="drill-${DEMO_NAME}-$(date +%s)"
  cat <<JSON | curl -fsS -X POST "$WEBHOOK_URL" -H 'content-type: application/json' -d @- >/dev/null
{
  "alerts": [
    {
      "status": "firing",
      "fingerprint": "${fingerprint}",
      "labels": {
        "workload_kind": "Deployment",
        "namespace": "${NAMESPACE}",
        "name": "${DEMO_NAME}",
        "alertname": "CrashLoopBackOff",
        "severity": "Critical"
      },
      "annotations": {
        "summary": "local runtime smoke"
      }
    }
  ]
}
JSON
}

wait_for_hr_creation() {
  wait_for_condition "HealingRequest creation" 30 kubectl -n "$NAMESPACE" get healingrequest "hr-${DEMO_NAME}"
}

assert_default_oncall_path() {
  wait_for_condition "default oncall state" "$BLOCK_TIMEOUT_SECONDS" bash -lc "phase=\$(kubectl -n '${NAMESPACE}' get healingrequest 'hr-${DEMO_NAME}' -o jsonpath='{.status.phase}' 2>/dev/null || true); [[ \"\$phase\" == 'PendingVerify' || \"\$phase\" == 'Blocked' || \"\$phase\" == 'L3' ]]"
  local phase block_reason gate_decision oncall_state described
  phase="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.phase}')"
  block_reason="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.blockReasonCode}')"
  gate_decision="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.lastGateDecision}')"
  oncall_state="$(oncall_state_from_phase "$phase")"
  described="$(kubectl -n "$NAMESPACE" describe healingrequest "hr-${DEMO_NAME}" 2>/dev/null || true)"
  info "default phase=${phase} oncallState=${oncall_state} blockReasonCode=${block_reason}"
  info "default decision=${gate_decision}"
  if [[ "$oncall_state" != "observing" && "$oncall_state" != "blocked" ]]; then
    fail "expected observing or blocked oncall state, got ${oncall_state} (phase=${phase})"
  fi
}

relax_request_for_single_success_path() {
  info "relaxing current HealingRequest for one local success pass"
  kubectl -n "$NAMESPACE" patch healingrequest "hr-${DEMO_NAME}" --type merge -p "{\"spec\":{\"blastRadius\":{\"maxPodPercentage\":${RELAXED_MAX_POD_PERCENT}},\"soakTimePolicies\":[{\"category\":\"CrashLoopBackOff\",\"severity\":\"Critical\",\"durationSec\":${SOAK_DURATION_SECONDS},\"minSamples\":${SOAK_MIN_SAMPLES}}]}}" >/dev/null
}

assert_success_path() {
  local pending_seen=false
  local deadline=$((SECONDS + SUCCESS_TIMEOUT_SECONDS))
  while (( SECONDS < deadline )); do
    local phase
    phase="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.phase}')"
    if [[ "$phase" == "PendingVerify" ]]; then
      pending_seen=true
    fi
    if [[ "$phase" == "Completed" || "$phase" == "PendingVerify" ]]; then
      break
    fi
    sleep "$POLL_INTERVAL_SECONDS"
  done

  local final_phase final_action gate_outcome oncall_state
  final_phase="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.phase}')"
  final_action="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.lastAction}')"
  gate_outcome="$(jsonpath_value "healingrequest/hr-${DEMO_NAME}" '{.status.gateOutcome}')"
  oncall_state="$(oncall_state_from_phase "$final_phase")"
  if [[ "$final_phase" != "Completed" && "$final_phase" != "PendingVerify" ]]; then
    fail "expected Completed or PendingVerify on relaxed path, got ${final_phase}"
  fi
  if [[ "$oncall_state" != "auto-tried" && "$oncall_state" != "observing" ]]; then
    fail "expected oncall state auto-tried or observing, got ${oncall_state}"
  fi
  if [[ "$pending_seen" == true && "$final_phase" == "PendingVerify" ]]; then
    info "relaxed path is still observing; oncall state remains coherent"
  elif [[ "$pending_seen" == true ]]; then
    info "observed PendingVerify before Completed"
  else
    info "relaxed path reached Completed directly within polling window"
  fi
  info "relaxed path lastAction=${final_action} gateOutcome=${gate_outcome} oncallState=${oncall_state}"
}

main() {
  info "[1/4] checking cluster prerequisites"
  ensure_prerequisites

  info "[2/4] resetting previous smoke state"
  cleanup_previous_state
  kubectl -n "$NAMESPACE" rollout status deployment "$DEMO_NAME" --timeout=90s >/dev/null

  info "[3/4] validating default oncall path"
  send_demo_alert
  wait_for_hr_creation
  assert_default_oncall_path

  info "[4/4] validating relaxed single-request success path"
  relax_request_for_single_success_path
  assert_success_path

  info "smoke completed: default oncall path and relaxed progression path both passed"
}

main "$@"
