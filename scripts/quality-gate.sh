#!/usr/bin/env bash
set -euo pipefail

TRACE_FILE="${QUALITY_GATE_TRACE_FILE:-}"
SLO_POLICY_FILE="${QUALITY_GATE_SLO_POLICY_FILE:-config/slo/runtime-slo-policy.yaml}"
ALERT_STATE_FILE="${QUALITY_GATE_ALERT_STATE_FILE:-}"
ALERT_SUPPRESSION_WINDOW_SECONDS="${QUALITY_GATE_SUPPRESSION_WINDOW_SECONDS:-600}"
ALERT_KEY="${QUALITY_GATE_ALERT_KEY:-global}"

normalize_outcome() {
  local value="${1:-degrade}"
  case "$value" in
    allow|block|degrade) echo "$value" ;;
    *) echo "degrade" ;;
  esac
}

derive_incident_level() {
  local outcome="$(normalize_outcome "$1")"
  case "$outcome" in
    allow) echo "info" ;;
    degrade) echo "warning" ;;
    *) echo "critical" ;;
  esac
}

derive_recovery_condition() {
  local outcome="$(normalize_outcome "$1")"
  case "$outcome" in
    allow) echo "maintain_target_and_observe" ;;
    degrade) echo "recover_budget_below_degrade_threshold" ;;
    *) echo "manual_approval_after_incident_review" ;;
  esac
}

derive_runbook() {
  local outcome="$(normalize_outcome "$1")"
  case "$outcome" in
    allow) echo "runbook://runtime-observation" ;;
    degrade) echo "runbook://runtime-degrade-recovery" ;;
    *) echo "runbook://runtime-block-rollback" ;;
  esac
}

derive_budget_status() {
  local outcome="$(normalize_outcome "$1")"
  case "$outcome" in
    allow) echo "healthy" ;;
    degrade) echo "warning" ;;
    *) echo "exhausted" ;;
  esac
}

emit_slo_fields() {
  local outcome="$(normalize_outcome "$1")"
  local budget_status="${QUALITY_GATE_SLO_BUDGET_STATUS:-$(derive_budget_status "$outcome")}"
  local incident_level="${QUALITY_GATE_INCIDENT_LEVEL:-$(derive_incident_level "$outcome")}"
  local recovery_condition="${QUALITY_GATE_RECOVERY_CONDITION:-$(derive_recovery_condition "$outcome")}"
  local runbook="${QUALITY_GATE_RUNBOOK:-$(derive_runbook "$outcome")}"

  echo "QUALITY_GATE_SLO_POLICY_FILE=${SLO_POLICY_FILE}"
  echo "QUALITY_GATE_SLO_ACTION_LEVEL=${outcome}"
  echo "QUALITY_GATE_SLO_BUDGET_STATUS=${budget_status}"
  echo "QUALITY_GATE_INCIDENT_LEVEL=${incident_level}"
  echo "QUALITY_GATE_RECOVERY_CONDITION=${recovery_condition}"
  echo "QUALITY_GATE_RUNBOOK=${runbook}"
}

assert_incident_level_mapping() {
  local outcome="$(normalize_outcome "$1")"
  local incident_level="$2"
  local expected="$(derive_incident_level "$outcome")"
  if [[ "$incident_level" != "$expected" ]]; then
    print_failure "incident_mapping" "incident_level_semantic_mismatch" "align QUALITY_GATE_SLO_ACTION_LEVEL and QUALITY_GATE_INCIDENT_LEVEL" "block"
    return 1
  fi
  return 0
}

upsert_alert_state() {
  local key="$1"
  local level="$2"
  local timestamp="$3"
  local suppressed_count="$4"
  local outcome="$5"
  local file="$6"
  local temp_file
  temp_file="$(mktemp)"
  if [[ -f "$file" ]]; then
    grep -v "^${key}|" "$file" >"$temp_file" || true
  fi
  echo "${key}|${level}|${timestamp}|${suppressed_count}|${outcome}" >>"$temp_file"
  mv "$temp_file" "$file"
}

emit_alert_notification_state() {
  local outcome="$(normalize_outcome "$1")"
  local incident_level="$2"
  local now_ts="$3"

  if [[ -z "$ALERT_STATE_FILE" ]]; then
    echo "QUALITY_GATE_ALERT_NOTIFY=true"
    echo "QUALITY_GATE_ALERT_SUPPRESSED_COUNT=0"
    return 0
  fi

  touch "$ALERT_STATE_FILE"
  local existing
  existing="$(grep -E "^${ALERT_KEY}\|" "$ALERT_STATE_FILE" || true)"
  local prev_level=""
  local prev_ts="0"
  local prev_count="0"
  local prev_outcome=""
  if [[ -n "$existing" ]]; then
    IFS='|' read -r _ prev_level prev_ts prev_count prev_outcome <<<"$existing"
  fi

  local notify="true"
  local next_count="0"
  if [[ "$prev_level" == "$incident_level" ]] && (( now_ts - prev_ts < ALERT_SUPPRESSION_WINDOW_SECONDS )); then
    notify="false"
    next_count=$((prev_count + 1))
  fi

  upsert_alert_state "$ALERT_KEY" "$incident_level" "$now_ts" "$next_count" "$outcome" "$ALERT_STATE_FILE"
  echo "QUALITY_GATE_ALERT_NOTIFY=${notify}"
  echo "QUALITY_GATE_ALERT_SUPPRESSED_COUNT=${next_count}"
}

append_trace() {
  local step="$1"
  if [[ -n "$TRACE_FILE" ]]; then
    echo "$step" >>"$TRACE_FILE"
  fi
}

print_failure() {
  local category="$1"
  local reason="$2"
  local fix_hint="$3"
  local outcome="${4:-block}"
  echo "QUALITY_GATE_RESULT=block"
  echo "QUALITY_GATE_CATEGORY=${category}"
  echo "QUALITY_GATE_REASON=${reason}"
  echo "QUALITY_GATE_FIX_HINT=${fix_hint}"
  emit_slo_fields "$outcome"
}

run_step() {
  local step="$1"
  local category="$2"
  local reason="$3"
  local fix_hint="$4"
  local cmd="$5"

  append_trace "$step"
  echo "[quality-gate] step=${step}"
  if ! bash -c "$cmd"; then
    print_failure "$category" "$reason" "$fix_hint" "block"
    return 1
  fi
}

assert_slo_consistency() {
  local gate_result="$(normalize_outcome "$1")"
  local slo_level="$(normalize_outcome "${QUALITY_GATE_SLO_ACTION_LEVEL:-$gate_result}")"
  if [[ "$gate_result" != "$slo_level" ]]; then
    print_failure "slo_governance" "slo_gate_semantic_mismatch" "align QUALITY_GATE_RESULT and QUALITY_GATE_SLO_ACTION_LEVEL" "block"
    return 1
  fi
  return 0
}

assert_recovery_ready() {
  local outcome="$(normalize_outcome "$1")"
  local recovery_ready="${QUALITY_GATE_RECOVERY_READY:-true}"
  if [[ "$outcome" == "allow" ]] && [[ "$recovery_ready" != "true" ]]; then
    print_failure "slo_recovery" "recovery_condition_not_met" "satisfy recovery condition before allow rollout" "block"
    return 1
  fi
  return 0
}

QUALITY_GATE_CMD_TEST="${QUALITY_GATE_CMD_TEST:-go test ./...}"
QUALITY_GATE_CMD_RACE="${QUALITY_GATE_CMD_RACE:-go test -race ./internal/controllers ./internal/healing ./internal/safety ./internal/ingestion ./internal/observability}"
QUALITY_GATE_CMD_VET="${QUALITY_GATE_CMD_VET:-go vet ./...}"
QUALITY_GATE_CMD_LINT="${QUALITY_GATE_CMD_LINT:-golangci-lint run}"
QUALITY_GATE_CMD_CRD_CHECK="${QUALITY_GATE_CMD_CRD_CHECK:-bash ./scripts/check-crd-consistency.sh}"
QUALITY_GATE_CMD_HELM_SYNC="${QUALITY_GATE_CMD_HELM_SYNC:-go test ./charts/kube-sentinel -run 'TestValuesSchemaIncludesProductionGatePolicy|TestValuesYamlIncludesProductionGatePolicyDefaults'}"

run_step "unit_test" "unit_test" "unit_tests_failed" "run: go test ./..." "$QUALITY_GATE_CMD_TEST"
run_step "race_core" "race" "race_detection_failed" "run: go test -race ./internal/..." "$QUALITY_GATE_CMD_RACE"
run_step "vet" "static_analysis" "go_vet_failed" "run: go vet ./..." "$QUALITY_GATE_CMD_VET"
run_step "lint" "lint" "golangci_lint_failed" "run: golangci-lint run" "$QUALITY_GATE_CMD_LINT"
run_step "crd_consistency" "crd_consistency" "crd_generation_drift" "run: bash ./scripts/check-crd-consistency.sh" "$QUALITY_GATE_CMD_CRD_CHECK"
run_step "helm_sync" "api_crd_helm_sync" "helm_constraints_mismatch" "run: go test ./charts/kube-sentinel" "$QUALITY_GATE_CMD_HELM_SYNC"

assert_slo_consistency "allow"
assert_recovery_ready "allow"

incident_level="${QUALITY_GATE_INCIDENT_LEVEL:-$(derive_incident_level "allow")}"
assert_incident_level_mapping "allow" "$incident_level"
emit_alert_notification_state "allow" "$incident_level" "$(date +%s)"

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=quality_gate"
echo "QUALITY_GATE_REASON=all_checks_passed"
emit_slo_fields "allow"
