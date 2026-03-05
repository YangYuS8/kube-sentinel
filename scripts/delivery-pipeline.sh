#!/usr/bin/env bash
set -euo pipefail

PIPELINE_WORK_DIR="${DELIVERY_PIPELINE_WORK_DIR:-$(mktemp -d)}"
PIPELINE_TRACE_FILE="${DELIVERY_PIPELINE_TRACE_FILE:-${PIPELINE_WORK_DIR}/delivery-pipeline.trace}"
QUALITY_OUTPUT_FILE="${DELIVERY_PIPELINE_QUALITY_OUTPUT_FILE:-${PIPELINE_WORK_DIR}/quality-gate.env}"
DRY_RUN_OUTPUT_FILE="${DELIVERY_PIPELINE_DRY_RUN_OUTPUT_FILE:-${PIPELINE_WORK_DIR}/dry-run.env}"
PIPELINE_EVIDENCE_JSON="${DELIVERY_PIPELINE_EVIDENCE_JSON:-${PIPELINE_WORK_DIR}/delivery-evidence.json}"
PIPELINE_SUMMARY_FILE="${DELIVERY_PIPELINE_SUMMARY_FILE:-${PIPELINE_WORK_DIR}/delivery-summary.txt}"
PIPELINE_ARCHIVE_DIR="${DELIVERY_PIPELINE_ARCHIVE_DIR:-${PIPELINE_WORK_DIR}/archive}"
OVERRIDE_IDEMPOTENCY_FILE="${DELIVERY_PIPELINE_OVERRIDE_IDEMPOTENCY_FILE:-${PIPELINE_WORK_DIR}/override-idempotency.log}"
OVERRIDE_AUDIT_FILE="${DELIVERY_PIPELINE_OVERRIDE_AUDIT_FILE:-${PIPELINE_WORK_DIR}/override-audit.log}"
GO_LIVE_DECISION_PACK_FILE="${DELIVERY_PIPELINE_DECISION_PACK_FILE:-${PIPELINE_WORK_DIR}/release-decision-pack.json}"

QUALITY_GATE_CMD="${DELIVERY_PIPELINE_QUALITY_GATE_CMD:-bash ./scripts/quality-gate.sh}"
DRY_RUN_CMD="${DELIVERY_PIPELINE_DRY_RUN_CMD:-printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=preprod_simulated\nDRY_RUN_TRACE_KEY=dryrun-default\n'}"

mkdir -p "${PIPELINE_WORK_DIR}" "${PIPELINE_ARCHIVE_DIR}"
: >"${PIPELINE_TRACE_FILE}"

append_trace() {
  local step="$1"
  echo "$step" >>"${PIPELINE_TRACE_FILE}"
}

normalize_outcome() {
  local value="${1:-degrade}"
  case "$value" in
    allow|degrade|block) echo "$value" ;;
    *) echo "invalid" ;;
  esac
}

derive_oncall_level() {
  case "$(normalize_outcome "$1")" in
    allow) echo "info" ;;
    degrade) echo "warning" ;;
    block) echo "critical" ;;
    *) echo "unknown" ;;
  esac
}

derive_oncall_runbook() {
  case "$(normalize_outcome "$1")" in
    allow) echo "runbook://runtime-observation" ;;
    degrade) echo "runbook://runtime-degrade-recovery" ;;
    block) echo "runbook://runtime-block-rollback" ;;
    *) echo "runbook://runtime-unknown" ;;
  esac
}

derive_oncall_approval() {
  case "$(normalize_outcome "$1")" in
    allow) echo "observe_only" ;;
    degrade) echo "oncall_ack_required" ;;
    block) echo "incident_commander_approval" ;;
    *) echo "unknown" ;;
  esac
}

normalize_gate_status() {
  local value="${1:-fail}"
  case "$value" in
    pass|fail) echo "$value" ;;
    *) echo "fail" ;;
  esac
}

normalize_bool() {
  local value="${1:-false}"
  case "$value" in
    true|false) echo "$value" ;;
    *) echo "false" ;;
  esac
}

normalize_decision() {
  local value="${1:-block}"
  case "$value" in
    allow|block) echo "$value" ;;
    *) echo "block" ;;
  esac
}

is_integer() {
  [[ "${1:-}" =~ ^-?[0-9]+$ ]]
}

is_non_negative_number() {
  [[ "${1:-}" =~ ^[0-9]+([.][0-9]+)?$ ]]
}

is_evidence_expired() {
  local now_ts="$1"
  local evidence_ts="$2"
  local ttl_seconds="$3"
  if ! is_integer "$now_ts" || ! is_integer "$evidence_ts" || ! is_integer "$ttl_seconds"; then
    return 0
  fi
  if (( ttl_seconds < 0 )); then
    return 0
  fi
  local age=$((now_ts - evidence_ts))
  if (( age > ttl_seconds )); then
    return 0
  fi
  return 1
}

is_within_freeze_window() {
  local now_ts="$1"
  local start_ts="$2"
  local end_ts="$3"
  if ! is_integer "$now_ts" || ! is_integer "$start_ts" || ! is_integer "$end_ts"; then
    return 1
  fi
  if (( start_ts > end_ts )); then
    return 1
  fi
  if (( now_ts >= start_ts && now_ts <= end_ts )); then
    return 0
  fi
  return 1
}

resolve_failure_category() {
  local quality_status="$1"
  local stability_status="$2"
  local drill_status="$3"
  local approval_status="$4"
  local audit_status="$5"
  if [[ "$quality_status" != "pass" ]]; then
    echo "quality"
    return 0
  fi
  if [[ "$stability_status" != "pass" ]]; then
    echo "stability"
    return 0
  fi
  if [[ "$drill_status" != "pass" ]]; then
    echo "drill_rollback"
    return 0
  fi
  if [[ "$approval_status" != "pass" ]]; then
    echo "approval_freeze"
    return 0
  fi
  if [[ "$audit_status" != "pass" ]]; then
    echo "audit_integrity"
    return 0
  fi
  echo "none"
}

ensure_decision_pack_required_fields() {
  local final_decision="$1"
  local failure_category="$2"
  local rollback_candidate="$3"
  local approval_status="$4"
  local correlation_key="$5"
  local decision_timestamp="$6"
  local missing=()

  if [[ -z "$final_decision" ]]; then
    missing+=("decision")
  fi
  if [[ -z "$failure_category" ]]; then
    missing+=("failureCategory")
  fi
  if [[ -z "$rollback_candidate" ]]; then
    missing+=("rollbackCandidate")
  fi
  if [[ -z "$approval_status" ]]; then
    missing+=("approvalStatus")
  fi
  if [[ -z "$correlation_key" ]]; then
    missing+=("correlationKey")
  fi
  if [[ -z "$decision_timestamp" ]]; then
    missing+=("timestamp")
  fi

  if (( ${#missing[@]} > 0 )); then
    echo "${missing[*]}"
    return 1
  fi
  return 0
}

extract_key() {
  local file="$1"
  local key="$2"
  if [[ ! -f "$file" ]]; then
    echo ""
    return 0
  fi
  local line
  line="$(grep -E "^${key}=" "$file" | tail -n 1 || true)"
  echo "${line#*=}"
}

validate_required_fields() {
  local file="$1"
  shift
  local missing=()
  local key
  for key in "$@"; do
    local value
    value="$(extract_key "$file" "$key")"
    if [[ -z "$value" ]]; then
      missing+=("$key")
    fi
  done
  if (( ${#missing[@]} > 0 )); then
    echo "missing_fields:${missing[*]}"
    return 1
  fi
  return 0
}

run_stage() {
  local stage="$1"
  local cmd="$2"
  local output_file="$3"
  append_trace "$stage"
  if ! bash -c "$cmd" | tee "$output_file"; then
    return 1
  fi
  return 0
}

write_failure_evidence() {
  local failed_stage="$1"
  local reason="$2"
  local suggestion="$3"
  cat >"${PIPELINE_EVIDENCE_JSON}" <<EOF
{
  "result": "block",
  "failedStage": "${failed_stage}",
  "category": "delivery_pipeline",
  "reasonCode": "${reason}",
  "fixHint": "${suggestion}",
  "goLiveDecision": "block",
  "traceFile": "${PIPELINE_TRACE_FILE}",
  "qualityOutputFile": "${QUALITY_OUTPUT_FILE}",
  "dryRunOutputFile": "${DRY_RUN_OUTPUT_FILE}",
  "decisionPackFile": "${GO_LIVE_DECISION_PACK_FILE}"
}
EOF
  cat >"${PIPELINE_SUMMARY_FILE}" <<EOF
DELIVERY_PIPELINE_RESULT=block
DELIVERY_PIPELINE_FAILED_STAGE=${failed_stage}
DELIVERY_PIPELINE_REASON=${reason}
DELIVERY_PIPELINE_FIX_HINT=${suggestion}
DELIVERY_PIPELINE_TRACE_FILE=${PIPELINE_TRACE_FILE}
DELIVERY_PIPELINE_DECISION=block
EOF
}

assert_oncall_consistency() {
  local decision="$1"
  local normalized
  normalized="$(normalize_outcome "$decision")"
  if [[ "$normalized" == "invalid" ]]; then
    return 1
  fi

  local expected_level expected_runbook expected_approval
  expected_level="$(derive_oncall_level "$normalized")"
  expected_runbook="$(derive_oncall_runbook "$normalized")"
  expected_approval="$(derive_oncall_approval "$normalized")"

  local actual_level actual_runbook actual_approval
  actual_level="${ONCALL_ALERT_LEVEL:-$expected_level}"
  actual_runbook="${ONCALL_RUNBOOK:-$expected_runbook}"
  actual_approval="${ONCALL_APPROVAL_TRIGGER:-$expected_approval}"

  if [[ "$actual_level" != "$expected_level" ]] || [[ "$actual_runbook" != "$expected_runbook" ]] || [[ "$actual_approval" != "$expected_approval" ]]; then
    return 1
  fi
  return 0
}

record_override_audit() {
  local pipeline_decision="$1"
  local override="${DELIVERY_PIPELINE_OPERATOR_OVERRIDE:-false}"
  local timestamp actor pre_decision post_decision reason trace_key

  if [[ "$override" != "true" ]]; then
    echo "false"
    return 0
  fi

  timestamp="${DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP:-}"
  actor="${DELIVERY_PIPELINE_OVERRIDE_ACTOR:-}"
  pre_decision="${DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION:-}"
  post_decision="${DELIVERY_PIPELINE_OVERRIDE_POST_DECISION:-}"
  reason="${DELIVERY_PIPELINE_OVERRIDE_REASON:-}"
  trace_key="${DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY:-}"

  if [[ -z "$timestamp" || -z "$actor" || -z "$pre_decision" || -z "$post_decision" || -z "$reason" || -z "$trace_key" ]]; then
    echo "missing"
    return 0
  fi

  mkdir -p "$(dirname "$OVERRIDE_IDEMPOTENCY_FILE")" "$(dirname "$OVERRIDE_AUDIT_FILE")"
  touch "$OVERRIDE_IDEMPOTENCY_FILE" "$OVERRIDE_AUDIT_FILE"

  if grep -Fxq "$trace_key" "$OVERRIDE_IDEMPOTENCY_FILE"; then
    echo "idempotent"
    return 0
  fi

  {
    echo "override.actor=${actor}"
    echo "override.preDecision=${pre_decision}"
    echo "override.postDecision=${post_decision}"
    echo "override.reason=${reason}"
    echo "override.timestamp=${timestamp}"
    echo "override.traceKey=${trace_key}"
    echo "override.pipelineDecision=${pipeline_decision}"
    echo "override.correlationKey=${trace_key}"
    echo "--"
  } >>"$OVERRIDE_AUDIT_FILE"

  echo "$trace_key" >>"$OVERRIDE_IDEMPOTENCY_FILE"
  echo "recorded"
}

pipeline_failed_stage=""
pipeline_reason=""
pipeline_fix_hint=""

if ! run_stage "quality_gate" "$QUALITY_GATE_CMD" "$QUALITY_OUTPUT_FILE"; then
  pipeline_failed_stage="quality_gate"
  pipeline_reason="quality_gate_failed"
  pipeline_fix_hint="fix quality gate failure and rerun delivery pipeline"
fi

if [[ -z "$pipeline_failed_stage" ]]; then
  if ! validate_required_fields "$QUALITY_OUTPUT_FILE" QUALITY_GATE_RESULT QUALITY_GATE_CATEGORY QUALITY_GATE_REASON QUALITY_GATE_FIX_HINT >/dev/null; then
    pipeline_failed_stage="quality_gate"
    pipeline_reason="quality_gate_evidence_missing_fields"
    pipeline_fix_hint="ensure quality gate output includes result/category/reason/fix hint"
  fi
fi

if [[ -z "$pipeline_failed_stage" ]]; then
  if ! run_stage "preprod_dry_run" "$DRY_RUN_CMD" "$DRY_RUN_OUTPUT_FILE"; then
    pipeline_failed_stage="preprod_dry_run"
    pipeline_reason="preprod_dry_run_failed"
    pipeline_fix_hint="fix dry-run command and rerun delivery pipeline"
  fi
fi

if [[ -z "$pipeline_failed_stage" ]]; then
  if ! validate_required_fields "$DRY_RUN_OUTPUT_FILE" DRY_RUN_OUTCOME DRY_RUN_REASON DRY_RUN_TRACE_KEY >/dev/null; then
    pipeline_failed_stage="preprod_dry_run"
    pipeline_reason="dry_run_evidence_missing_fields"
    pipeline_fix_hint="ensure dry-run output includes outcome/reason/trace key"
  fi
fi

quality_result="$(normalize_outcome "$(extract_key "$QUALITY_OUTPUT_FILE" "QUALITY_GATE_RESULT")")"
release_decision="$(normalize_outcome "$(extract_key "$QUALITY_OUTPUT_FILE" "QUALITY_GATE_RELEASE_DECISION")")"
if [[ "$release_decision" == "invalid" ]]; then
  release_decision="$quality_result"
fi

dry_run_outcome="$(normalize_outcome "$(extract_key "$DRY_RUN_OUTPUT_FILE" "DRY_RUN_OUTCOME")")"
if [[ -z "$pipeline_failed_stage" ]] && [[ "$dry_run_outcome" == "invalid" ]]; then
  pipeline_failed_stage="preprod_dry_run"
  pipeline_reason="dry_run_outcome_invalid"
  pipeline_fix_hint="set DRY_RUN_OUTCOME to allow|degrade|block"
fi

if [[ -z "$pipeline_failed_stage" ]]; then
  if ! assert_oncall_consistency "$release_decision"; then
    pipeline_failed_stage="oncall_consistency"
    pipeline_reason="release_oncall_template_conflict"
    pipeline_fix_hint="align oncall level/runbook/approval with release decision"
  fi
fi

override_state="false"
if [[ -z "$pipeline_failed_stage" ]]; then
  override_state="$(record_override_audit "$release_decision")"
  if [[ "$override_state" == "missing" ]]; then
    pipeline_failed_stage="operator_override_audit"
    pipeline_reason="operator_override_fields_missing"
    pipeline_fix_hint="fill override actor/pre/post/reason/timestamp/trace key"
  fi
fi

now_epoch="${DELIVERY_PIPELINE_NOW_EPOCH_SECONDS:-$(date +%s)}"
preprod_status="$(normalize_outcome "${DELIVERY_PIPELINE_PREPROD_STATUS:-$dry_run_outcome}")"
preprod_evidence_epoch="${DELIVERY_PIPELINE_PREPROD_EVIDENCE_TIMESTAMP_EPOCH_SECONDS:-$now_epoch}"
preprod_evidence_ttl_seconds="${DELIVERY_PIPELINE_PREPROD_EVIDENCE_TTL_SECONDS:-3600}"

drill_success_rate="$(extract_key "$QUALITY_OUTPUT_FILE" "QUALITY_GATE_DRILL_SUCCESS_RATE")"
if [[ -z "$drill_success_rate" ]]; then
  drill_success_rate="${QUALITY_GATE_DRILL_SUCCESS_RATE:-1.0}"
fi
drill_min_success_rate="${DELIVERY_PIPELINE_DRILL_MIN_SUCCESS_RATE:-0.95}"
drill_rollback_p95_ms="$(extract_key "$QUALITY_OUTPUT_FILE" "QUALITY_GATE_DRILL_ROLLBACK_P95_MS")"
if [[ -z "$drill_rollback_p95_ms" ]]; then
  drill_rollback_p95_ms="${QUALITY_GATE_DRILL_ROLLBACK_P95_MS:-0}"
fi
drill_max_rollback_p95_ms="${DELIVERY_PIPELINE_DRILL_MAX_ROLLBACK_P95_MS:-300000}"

freeze_window_start_epoch="${DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS:-0}"
freeze_window_end_epoch="${DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS:-0}"

required_approval="$(derive_oncall_approval "$release_decision")"
approval_level="${ONCALL_APPROVAL_LEVEL:-${ONCALL_APPROVAL_TRIGGER:-$required_approval}}"

rollback_candidate="$(extract_key "$QUALITY_OUTPUT_FILE" "QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE")"
if [[ -z "$rollback_candidate" ]]; then
  rollback_candidate="latest-healthy-revision"
fi
decision_timestamp="${DELIVERY_PIPELINE_DECISION_TIMESTAMP:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
correlation_key="${DELIVERY_PIPELINE_CORRELATION_KEY:-$(extract_key "$DRY_RUN_OUTPUT_FILE" "DRY_RUN_TRACE_KEY")}"

quality_gate_status="pass"
quality_gate_reason="ok"
stability_gate_status="pass"
stability_gate_reason="ok"
drill_gate_status="pass"
drill_gate_reason="ok"
approval_freeze_gate_status="pass"
approval_freeze_gate_reason="ok"
audit_integrity_gate_status="pass"
audit_integrity_gate_reason="ok"

if [[ "$quality_result" != "allow" || "$release_decision" != "allow" ]]; then
  quality_gate_status="fail"
  quality_gate_reason="quality_or_release_not_allow"
fi

if [[ "$preprod_status" != "allow" ]]; then
  stability_gate_status="fail"
  stability_gate_reason="preprod_not_passed"
elif is_evidence_expired "$now_epoch" "$preprod_evidence_epoch" "$preprod_evidence_ttl_seconds"; then
  stability_gate_status="fail"
  stability_gate_reason="preprod_evidence_expired"
fi

if ! is_non_negative_number "$drill_success_rate" || ! is_non_negative_number "$drill_min_success_rate"; then
  drill_gate_status="fail"
  drill_gate_reason="drill_success_rate_invalid"
elif ! awk -v actual="$drill_success_rate" -v threshold="$drill_min_success_rate" 'BEGIN { exit (actual+0 >= threshold+0) ? 0 : 1 }'; then
  drill_gate_status="fail"
  drill_gate_reason="drill_success_rate_below_threshold"
elif ! is_integer "$drill_rollback_p95_ms" || ! is_integer "$drill_max_rollback_p95_ms"; then
  drill_gate_status="fail"
  drill_gate_reason="drill_rollback_latency_invalid"
elif (( drill_rollback_p95_ms > drill_max_rollback_p95_ms )); then
  drill_gate_status="fail"
  drill_gate_reason="drill_rollback_latency_above_threshold"
fi

freeze_window_active="false"
if is_within_freeze_window "$now_epoch" "$freeze_window_start_epoch" "$freeze_window_end_epoch"; then
  freeze_window_active="true"
fi

if [[ "$approval_level" != "$required_approval" ]]; then
  approval_freeze_gate_status="fail"
  approval_freeze_gate_reason="approval_level_mismatch"
fi
if [[ "$freeze_window_active" == "true" && "${DELIVERY_PIPELINE_OPERATOR_OVERRIDE:-false}" == "true" ]]; then
  approval_freeze_gate_status="fail"
  approval_freeze_gate_reason="override_blocked_in_freeze_window"
fi

if [[ "$override_state" == "missing" ]]; then
  audit_integrity_gate_status="fail"
  audit_integrity_gate_reason="override_audit_fields_missing"
fi
if [[ -z "$rollback_candidate" ]]; then
  audit_integrity_gate_status="fail"
  audit_integrity_gate_reason="decision_pack_missing_rollback_candidate"
fi

go_live_failure_category="$(resolve_failure_category "$quality_gate_status" "$stability_gate_status" "$drill_gate_status" "$approval_freeze_gate_status" "$audit_integrity_gate_status")"
go_live_decision="allow"
if [[ "$go_live_failure_category" != "none" ]]; then
  go_live_decision="block"
fi

decision_pack_missing_fields=""
if ! decision_pack_missing_fields="$(ensure_decision_pack_required_fields "$go_live_decision" "$go_live_failure_category" "$rollback_candidate" "$approval_level" "$correlation_key" "$decision_timestamp")"; then
  go_live_decision="block"
  go_live_failure_category="audit_integrity"
  audit_integrity_gate_status="fail"
  audit_integrity_gate_reason="decision_pack_missing_fields"
  if [[ -z "$pipeline_failed_stage" ]]; then
    pipeline_reason="decision_pack_missing_fields"
    pipeline_fix_hint="ensure release decision pack required fields are populated"
  fi
fi

if [[ -n "$pipeline_failed_stage" ]]; then
  write_failure_evidence "$pipeline_failed_stage" "$pipeline_reason" "$pipeline_fix_hint"
  echo "DELIVERY_PIPELINE_RESULT=block"
  echo "DELIVERY_PIPELINE_FAILED_STAGE=${pipeline_failed_stage}"
  echo "DELIVERY_PIPELINE_REASON=${pipeline_reason}"
  echo "DELIVERY_PIPELINE_FIX_HINT=${pipeline_fix_hint}"
  echo "DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=${override_state}"
  echo "DELIVERY_PIPELINE_EVIDENCE_JSON=${PIPELINE_EVIDENCE_JSON}"
  echo "DELIVERY_PIPELINE_SUMMARY_FILE=${PIPELINE_SUMMARY_FILE}"
  exit 1
fi

final_result="$(normalize_decision "$go_live_decision")"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
archive_run_dir="${PIPELINE_ARCHIVE_DIR}/run-${timestamp}"
mkdir -p "$archive_run_dir"

cat >"${GO_LIVE_DECISION_PACK_FILE}" <<EOF
{
  "decision": "${final_result}",
  "failureCategory": "${go_live_failure_category}",
  "failedGateReason": "$(resolve_failure_category "$quality_gate_status" "$stability_gate_status" "$drill_gate_status" "$approval_freeze_gate_status" "$audit_integrity_gate_status")",
  "rollbackCandidate": "${rollback_candidate}",
  "drillSummary": {
    "successRate": "${drill_success_rate}",
    "minSuccessRate": "${drill_min_success_rate}",
    "rollbackP95Ms": "${drill_rollback_p95_ms}",
    "maxRollbackP95Ms": "${drill_max_rollback_p95_ms}",
    "gateStatus": "${drill_gate_status}",
    "gateReason": "${drill_gate_reason}"
  },
  "approval": {
    "requiredLevel": "${required_approval}",
    "providedLevel": "${approval_level}",
    "status": "${approval_freeze_gate_status}",
    "reason": "${approval_freeze_gate_reason}"
  },
  "freezeWindow": {
    "active": "${freeze_window_active}",
    "startEpochSeconds": "${freeze_window_start_epoch}",
    "endEpochSeconds": "${freeze_window_end_epoch}"
  },
  "gateResults": {
    "quality": {"status": "${quality_gate_status}", "reason": "${quality_gate_reason}"},
    "stability": {"status": "${stability_gate_status}", "reason": "${stability_gate_reason}"},
    "drillRollback": {"status": "${drill_gate_status}", "reason": "${drill_gate_reason}"},
    "approvalFreeze": {"status": "${approval_freeze_gate_status}", "reason": "${approval_freeze_gate_reason}"},
    "auditIntegrity": {"status": "${audit_integrity_gate_status}", "reason": "${audit_integrity_gate_reason}"}
  },
  "operatorOverride": {
    "enabled": "${DELIVERY_PIPELINE_OPERATOR_OVERRIDE:-false}",
    "auditState": "${override_state}"
  },
  "correlationKey": "${correlation_key}",
  "timestamp": "${decision_timestamp}"
}
EOF

cat >"${PIPELINE_EVIDENCE_JSON}" <<EOF
{
  "result": "${final_result}",
  "failedStage": "none",
  "category": "delivery_pipeline",
  "reasonCode": "all_stages_completed",
  "fixHint": "n/a",
  "goLiveDecision": "${final_result}",
  "goLiveFailureCategory": "${go_live_failure_category}",
  "qualityResult": "${quality_result}",
  "releaseDecision": "${release_decision}",
  "dryRunOutcome": "${dry_run_outcome}",
  "oncallLevel": "$(derive_oncall_level "$release_decision")",
  "oncallRunbook": "$(derive_oncall_runbook "$release_decision")",
  "oncallApprovalTrigger": "$(derive_oncall_approval "$release_decision")",
  "operatorOverride": "${DELIVERY_PIPELINE_OPERATOR_OVERRIDE:-false}",
  "operatorOverrideAuditState": "${override_state}",
  "operatorOverrideAuditFile": "${OVERRIDE_AUDIT_FILE}",
  "goLiveDecisionPackFile": "${GO_LIVE_DECISION_PACK_FILE}",
  "traceFile": "${PIPELINE_TRACE_FILE}",
  "qualityOutputFile": "${QUALITY_OUTPUT_FILE}",
  "dryRunOutputFile": "${DRY_RUN_OUTPUT_FILE}",
  "archiveDir": "${archive_run_dir}"
}
EOF

cat >"${PIPELINE_SUMMARY_FILE}" <<EOF
DELIVERY_PIPELINE_RESULT=${final_result}
DELIVERY_PIPELINE_FAILED_STAGE=none
DELIVERY_PIPELINE_REASON=all_stages_completed
DELIVERY_PIPELINE_DECISION=${final_result}
DELIVERY_PIPELINE_FAILURE_CATEGORY=${go_live_failure_category}
DELIVERY_PIPELINE_GATE_QUALITY_STATUS=${quality_gate_status}
DELIVERY_PIPELINE_GATE_STABILITY_STATUS=${stability_gate_status}
DELIVERY_PIPELINE_GATE_DRILL_ROLLBACK_STATUS=${drill_gate_status}
DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=${approval_freeze_gate_status}
DELIVERY_PIPELINE_GATE_AUDIT_INTEGRITY_STATUS=${audit_integrity_gate_status}
DELIVERY_PIPELINE_QUALITY_RESULT=${quality_result}
DELIVERY_PIPELINE_RELEASE_DECISION=${release_decision}
DELIVERY_PIPELINE_DRY_RUN_OUTCOME=${dry_run_outcome}
DELIVERY_PIPELINE_ONCALL_LEVEL=$(derive_oncall_level "$release_decision")
DELIVERY_PIPELINE_ONCALL_RUNBOOK=$(derive_oncall_runbook "$release_decision")
DELIVERY_PIPELINE_ONCALL_APPROVAL_TRIGGER=$(derive_oncall_approval "$release_decision")
DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=${override_state}
DELIVERY_PIPELINE_TRACE_FILE=${PIPELINE_TRACE_FILE}
DELIVERY_PIPELINE_EVIDENCE_JSON=${PIPELINE_EVIDENCE_JSON}
DELIVERY_PIPELINE_SUMMARY_FILE=${PIPELINE_SUMMARY_FILE}
DELIVERY_PIPELINE_DECISION_PACK_FILE=${GO_LIVE_DECISION_PACK_FILE}
EOF

append_trace "archive_evidence"
cp "$QUALITY_OUTPUT_FILE" "$archive_run_dir/quality-gate.env"
cp "$DRY_RUN_OUTPUT_FILE" "$archive_run_dir/dry-run.env"
cp "$PIPELINE_EVIDENCE_JSON" "$archive_run_dir/delivery-evidence.json"
cp "$PIPELINE_SUMMARY_FILE" "$archive_run_dir/delivery-summary.txt"
cp "$GO_LIVE_DECISION_PACK_FILE" "$archive_run_dir/release-decision-pack.json"
cp "$PIPELINE_TRACE_FILE" "$archive_run_dir/delivery-pipeline.trace"
if [[ -f "$OVERRIDE_AUDIT_FILE" ]]; then
  cp "$OVERRIDE_AUDIT_FILE" "$archive_run_dir/override-audit.log"
fi

echo "DELIVERY_PIPELINE_RESULT=${final_result}"
echo "DELIVERY_PIPELINE_FAILED_STAGE=none"
echo "DELIVERY_PIPELINE_REASON=all_stages_completed"
echo "DELIVERY_PIPELINE_DECISION=${final_result}"
echo "DELIVERY_PIPELINE_FAILURE_CATEGORY=${go_live_failure_category}"
echo "DELIVERY_PIPELINE_GATE_QUALITY_STATUS=${quality_gate_status}"
echo "DELIVERY_PIPELINE_GATE_STABILITY_STATUS=${stability_gate_status}"
echo "DELIVERY_PIPELINE_GATE_DRILL_ROLLBACK_STATUS=${drill_gate_status}"
echo "DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=${approval_freeze_gate_status}"
echo "DELIVERY_PIPELINE_GATE_AUDIT_INTEGRITY_STATUS=${audit_integrity_gate_status}"
echo "DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=${override_state}"
echo "DELIVERY_PIPELINE_ARCHIVE_DIR=${archive_run_dir}"
echo "DELIVERY_PIPELINE_EVIDENCE_JSON=${PIPELINE_EVIDENCE_JSON}"
echo "DELIVERY_PIPELINE_SUMMARY_FILE=${PIPELINE_SUMMARY_FILE}"
echo "DELIVERY_PIPELINE_DECISION_PACK_FILE=${GO_LIVE_DECISION_PACK_FILE}"
