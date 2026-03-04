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

normalize_compatibility_class() {
  local value="${1:-backward-compatible}"
  case "$value" in
    backward-compatible|migration-required|version-bump-required) echo "$value" ;;
    *) echo "invalid" ;;
  esac
}

normalize_risk_level() {
  local value="${1:-low}"
  case "$value" in
    low|medium|high) echo "$value" ;;
    *) echo "invalid" ;;
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

emit_api_contract_fields() {
  local compatibility_class="$(normalize_compatibility_class "${QUALITY_GATE_API_COMPATIBILITY_CLASS:-backward-compatible}")"
  local risk_level="$(normalize_risk_level "${QUALITY_GATE_API_RISK_LEVEL:-low}")"
  local migration_plan="${QUALITY_GATE_API_MIGRATION_PLAN:-}"
  local version_bump_window="${QUALITY_GATE_VERSION_BUMP_WINDOW:-}"
  local affected_fields="${QUALITY_GATE_API_AFFECTED_FIELDS:-api/v1alpha1/*}"
  local release_decision="${QUALITY_GATE_RELEASE_DECISION:-allow}"

  echo "QUALITY_GATE_API_COMPATIBILITY_CLASS=${compatibility_class}"
  echo "QUALITY_GATE_API_AFFECTED_FIELDS=${affected_fields}"
  if [[ -n "$migration_plan" ]]; then
    echo "QUALITY_GATE_API_MIGRATION_PLAN=${migration_plan}"
  else
    echo "QUALITY_GATE_API_MIGRATION_PLAN=<none>"
  fi
  if [[ -n "$version_bump_window" ]]; then
    echo "QUALITY_GATE_VERSION_BUMP_WINDOW=${version_bump_window}"
  else
    echo "QUALITY_GATE_VERSION_BUMP_WINDOW=<none>"
  fi
  echo "QUALITY_GATE_API_RISK_LEVEL=${risk_level}"
  echo "QUALITY_GATE_RELEASE_DECISION=${release_decision}"
}

normalize_bool() {
  local value="${1:-false}"
  case "$value" in
    true|false) echo "$value" ;;
    *) echo "false" ;;
  esac
}

emit_release_readiness_fields() {
  local action_type="${QUALITY_GATE_RELEASE_READINESS_ACTION_TYPE:-restart}"
  local risk_level="${QUALITY_GATE_RELEASE_READINESS_RISK_LEVEL:-low}"
  local strategy_mode="${QUALITY_GATE_RELEASE_READINESS_STRATEGY_MODE:-auto}"
  local circuit_tier="${QUALITY_GATE_RELEASE_READINESS_CIRCUIT_TIER:-none}"
  local operator_override="$(normalize_bool "${QUALITY_GATE_RELEASE_READINESS_OPERATOR_OVERRIDE:-false}")"
  local rollback_candidate="${QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE:-latest-healthy-revision}"
  local open_incidents="${QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS:-0}"
  local recent_drill_score="${QUALITY_GATE_RELEASE_READINESS_RECENT_DRILL_SCORE:-1.0}"
  local drill_success_rate="${QUALITY_GATE_DRILL_SUCCESS_RATE:-1.0}"
  local drill_rollback_p95_ms="${QUALITY_GATE_DRILL_ROLLBACK_P95_MS:-0}"
  local drill_gate_bypass_count="${QUALITY_GATE_DRILL_GATE_BYPASS_COUNT:-0}"
  local readiness_decision="$(normalize_outcome "${QUALITY_GATE_RELEASE_READINESS_DECISION:-${QUALITY_GATE_RELEASE_DECISION:-allow}}")"
  echo "QUALITY_GATE_RELEASE_READINESS_ACTION_TYPE=${action_type}"
  echo "QUALITY_GATE_RELEASE_READINESS_RISK_LEVEL=${risk_level}"
  echo "QUALITY_GATE_RELEASE_READINESS_STRATEGY_MODE=${strategy_mode}"
  echo "QUALITY_GATE_RELEASE_READINESS_CIRCUIT_TIER=${circuit_tier}"
  echo "QUALITY_GATE_RELEASE_READINESS_OPERATOR_OVERRIDE=${operator_override}"
  echo "QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=${rollback_candidate}"
  echo "QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS=${open_incidents}"
  echo "QUALITY_GATE_RELEASE_READINESS_RECENT_DRILL_SCORE=${recent_drill_score}"
  echo "QUALITY_GATE_DRILL_SUCCESS_RATE=${drill_success_rate}"
  echo "QUALITY_GATE_DRILL_ROLLBACK_P95_MS=${drill_rollback_p95_ms}"
  echo "QUALITY_GATE_DRILL_GATE_BYPASS_COUNT=${drill_gate_bypass_count}"
  echo "QUALITY_GATE_RELEASE_READINESS_DECISION=${readiness_decision}"
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
  emit_api_contract_fields
  emit_release_readiness_fields
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

assert_api_contract_consistency() {
  local compatibility_class="$(normalize_compatibility_class "${QUALITY_GATE_API_COMPATIBILITY_CLASS:-backward-compatible}")"
  local risk_level="$(normalize_risk_level "${QUALITY_GATE_API_RISK_LEVEL:-low}")"
  local migration_plan="${QUALITY_GATE_API_MIGRATION_PLAN:-}"
  local version_bump_window="${QUALITY_GATE_VERSION_BUMP_WINDOW:-}"
  if [[ "$compatibility_class" == "invalid" ]]; then
    print_failure "api_contract" "invalid_compatibility_class" "set QUALITY_GATE_API_COMPATIBILITY_CLASS to backward-compatible|migration-required|version-bump-required" "block"
    return 1
  fi
  if [[ "$risk_level" == "invalid" ]]; then
    print_failure "api_contract" "invalid_risk_level" "set QUALITY_GATE_API_RISK_LEVEL to low|medium|high" "block"
    return 1
  fi
  if [[ "$compatibility_class" == "migration-required" ]] && [[ -z "$migration_plan" ]]; then
    print_failure "api_contract" "migration_plan_missing" "set QUALITY_GATE_API_MIGRATION_PLAN for migration-required changes" "block"
    return 1
  fi
  if [[ "$compatibility_class" == "version-bump-required" ]] && [[ -z "$version_bump_window" ]]; then
    print_failure "api_contract" "version_bump_window_missing" "set QUALITY_GATE_VERSION_BUMP_WINDOW for version-bump-required changes" "block"
    return 1
  fi
  return 0
}

assert_release_gate_contract_binding() {
  local compatibility_class="$(normalize_compatibility_class "${QUALITY_GATE_API_COMPATIBILITY_CLASS:-backward-compatible}")"
  local risk_level="$(normalize_risk_level "${QUALITY_GATE_API_RISK_LEVEL:-low}")"
  local release_window_approved="${QUALITY_GATE_RELEASE_WINDOW_APPROVED:-false}"
  local migration_ready="${QUALITY_GATE_MIGRATION_READY:-false}"

  if [[ "$compatibility_class" == "migration-required" ]] && [[ "$migration_ready" != "true" ]]; then
    print_failure "runtime_production_gating" "migration_condition_not_met" "set QUALITY_GATE_MIGRATION_READY=true after validating migration preconditions" "block"
    return 1
  fi
  if [[ "$compatibility_class" == "version-bump-required" ]] && [[ "$release_window_approved" != "true" ]]; then
    print_failure "runtime_production_gating" "version_bump_window_not_approved" "set QUALITY_GATE_RELEASE_WINDOW_APPROVED=true for approved version bump window" "block"
    return 1
  fi
  if [[ "$risk_level" == "high" ]] && [[ "$release_window_approved" != "true" ]]; then
    print_failure "runtime_production_gating" "high_risk_release_not_approved" "set QUALITY_GATE_RELEASE_WINDOW_APPROVED=true for high-risk api contract changes" "block"
    return 1
  fi
  export QUALITY_GATE_RELEASE_DECISION="allow"
  return 0
}

assert_release_readiness_contract() {
  local rollback_candidate="${QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE:-latest-healthy-revision}"
  local open_incidents="${QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS:-0}"
  local max_open_incidents="${QUALITY_GATE_RELEASE_MAX_OPEN_INCIDENTS:-3}"
  local readiness_decision="$(normalize_outcome "${QUALITY_GATE_RELEASE_READINESS_DECISION:-${QUALITY_GATE_RELEASE_DECISION:-allow}}")"
  local release_decision="$(normalize_outcome "${QUALITY_GATE_RELEASE_DECISION:-allow}")"
  local allowed_decisions="${QUALITY_GATE_ALLOWED_DECISIONS:-allow,degrade,block}"

  if [[ -z "$rollback_candidate" ]]; then
    print_failure "runtime_production_gating" "release_readiness_missing_rollback_candidate" "set QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE to a validated healthy revision" "block"
    return 1
  fi
  if ! [[ "$open_incidents" =~ ^[0-9]+$ ]]; then
    print_failure "runtime_production_gating" "release_readiness_open_incidents_invalid" "set QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS to an integer" "block"
    return 1
  fi
  if ! [[ "$max_open_incidents" =~ ^[0-9]+$ ]]; then
    print_failure "runtime_production_gating" "release_readiness_max_open_incidents_invalid" "set QUALITY_GATE_RELEASE_MAX_OPEN_INCIDENTS to an integer" "block"
    return 1
  fi
  if (( open_incidents > max_open_incidents )); then
    print_failure "runtime_production_gating" "release_readiness_open_incidents_exceeded" "reduce active incidents or increase QUALITY_GATE_RELEASE_MAX_OPEN_INCIDENTS via approved policy" "block"
    return 1
  fi
  if [[ "$readiness_decision" != "$release_decision" ]]; then
    print_failure "runtime_production_gating" "release_readiness_decision_mismatch" "align QUALITY_GATE_RELEASE_READINESS_DECISION with QUALITY_GATE_RELEASE_DECISION" "block"
    return 1
  fi
  case ",$allowed_decisions," in
    *",${release_decision},"*) ;;
    *)
      print_failure "api_crd_helm_sync" "release_decision_not_allowed_by_policy" "set QUALITY_GATE_ALLOWED_DECISIONS to include allow,degrade,block semantics" "block"
      return 1
      ;;
  esac
  return 0
}

QUALITY_GATE_CMD_TEST="${QUALITY_GATE_CMD_TEST:-go test ./...}"
QUALITY_GATE_CMD_RACE="${QUALITY_GATE_CMD_RACE:-go test -race ./internal/controllers ./internal/healing ./internal/safety ./internal/ingestion ./internal/observability}"
QUALITY_GATE_CMD_VET="${QUALITY_GATE_CMD_VET:-go vet ./...}"
QUALITY_GATE_CMD_LINT="${QUALITY_GATE_CMD_LINT:-golangci-lint run}"
QUALITY_GATE_CMD_CRD_CHECK="${QUALITY_GATE_CMD_CRD_CHECK:-bash ./scripts/check-crd-consistency.sh}"
QUALITY_GATE_CMD_API_CONTRACT_SYNC="${QUALITY_GATE_CMD_API_CONTRACT_SYNC:-go test ./charts/kube-sentinel -run 'TestValuesSchemaIncludesProductionGatePolicy|TestValuesYamlIncludesProductionGatePolicyDefaults|TestValuesSchemaIncludesAPIContractPolicy|TestValuesYamlIncludesAPIContractPolicyDefaults|TestValuesSchemaIncludesReleaseReadinessPolicy|TestValuesYamlIncludesReleaseReadinessPolicyDefaults'}"
QUALITY_GATE_CMD_HELM_SYNC="${QUALITY_GATE_CMD_HELM_SYNC:-go test ./charts/kube-sentinel -run 'TestValuesSchemaIncludesProductionGatePolicy|TestValuesYamlIncludesProductionGatePolicyDefaults|TestValuesSchemaIncludesAPIContractPolicy|TestValuesYamlIncludesAPIContractPolicyDefaults|TestValuesSchemaIncludesReleaseReadinessPolicy|TestValuesYamlIncludesReleaseReadinessPolicyDefaults'}"

run_step "unit_test" "unit_test" "unit_tests_failed" "run: go test ./..." "$QUALITY_GATE_CMD_TEST"
run_step "race_core" "race" "race_detection_failed" "run: go test -race ./internal/..." "$QUALITY_GATE_CMD_RACE"
run_step "vet" "static_analysis" "go_vet_failed" "run: go vet ./..." "$QUALITY_GATE_CMD_VET"
run_step "lint" "lint" "golangci_lint_failed" "run: golangci-lint run" "$QUALITY_GATE_CMD_LINT"
run_step "crd_consistency" "crd_consistency" "crd_generation_drift" "run: bash ./scripts/check-crd-consistency.sh" "$QUALITY_GATE_CMD_CRD_CHECK"
run_step "api_contract_sync" "api_crd_helm_sync" "api_contract_sync_mismatch" "run: go test ./charts/kube-sentinel" "$QUALITY_GATE_CMD_API_CONTRACT_SYNC"
run_step "helm_sync" "api_crd_helm_sync" "helm_constraints_mismatch" "run: go test ./charts/kube-sentinel" "$QUALITY_GATE_CMD_HELM_SYNC"

assert_slo_consistency "allow"
assert_recovery_ready "allow"
assert_api_contract_consistency
assert_release_gate_contract_binding
assert_release_readiness_contract

incident_level="${QUALITY_GATE_INCIDENT_LEVEL:-$(derive_incident_level "allow")}"
assert_incident_level_mapping "allow" "$incident_level"
emit_alert_notification_state "allow" "$incident_level" "$(date +%s)"

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=quality_gate"
echo "QUALITY_GATE_REASON=all_checks_passed"
emit_slo_fields "allow"
emit_api_contract_fields
emit_release_readiness_fields
