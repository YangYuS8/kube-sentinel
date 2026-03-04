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
  "traceFile": "${PIPELINE_TRACE_FILE}",
  "qualityOutputFile": "${QUALITY_OUTPUT_FILE}",
  "dryRunOutputFile": "${DRY_RUN_OUTPUT_FILE}"
}
EOF
  cat >"${PIPELINE_SUMMARY_FILE}" <<EOF
DELIVERY_PIPELINE_RESULT=block
DELIVERY_PIPELINE_FAILED_STAGE=${failed_stage}
DELIVERY_PIPELINE_REASON=${reason}
DELIVERY_PIPELINE_FIX_HINT=${suggestion}
DELIVERY_PIPELINE_TRACE_FILE=${PIPELINE_TRACE_FILE}
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

final_result="allow"
if [[ "$quality_result" == "block" || "$release_decision" == "block" || "$dry_run_outcome" == "block" ]]; then
  final_result="block"
elif [[ "$quality_result" == "degrade" || "$release_decision" == "degrade" || "$dry_run_outcome" == "degrade" ]]; then
  final_result="degrade"
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
archive_run_dir="${PIPELINE_ARCHIVE_DIR}/run-${timestamp}"
mkdir -p "$archive_run_dir"

cat >"${PIPELINE_EVIDENCE_JSON}" <<EOF
{
  "result": "${final_result}",
  "failedStage": "none",
  "category": "delivery_pipeline",
  "reasonCode": "all_stages_completed",
  "fixHint": "n/a",
  "qualityResult": "${quality_result}",
  "releaseDecision": "${release_decision}",
  "dryRunOutcome": "${dry_run_outcome}",
  "oncallLevel": "$(derive_oncall_level "$release_decision")",
  "oncallRunbook": "$(derive_oncall_runbook "$release_decision")",
  "oncallApprovalTrigger": "$(derive_oncall_approval "$release_decision")",
  "operatorOverride": "${DELIVERY_PIPELINE_OPERATOR_OVERRIDE:-false}",
  "operatorOverrideAuditState": "${override_state}",
  "operatorOverrideAuditFile": "${OVERRIDE_AUDIT_FILE}",
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
EOF

append_trace "archive_evidence"
cp "$QUALITY_OUTPUT_FILE" "$archive_run_dir/quality-gate.env"
cp "$DRY_RUN_OUTPUT_FILE" "$archive_run_dir/dry-run.env"
cp "$PIPELINE_EVIDENCE_JSON" "$archive_run_dir/delivery-evidence.json"
cp "$PIPELINE_SUMMARY_FILE" "$archive_run_dir/delivery-summary.txt"
cp "$PIPELINE_TRACE_FILE" "$archive_run_dir/delivery-pipeline.trace"
if [[ -f "$OVERRIDE_AUDIT_FILE" ]]; then
  cp "$OVERRIDE_AUDIT_FILE" "$archive_run_dir/override-audit.log"
fi

echo "DELIVERY_PIPELINE_RESULT=${final_result}"
echo "DELIVERY_PIPELINE_FAILED_STAGE=none"
echo "DELIVERY_PIPELINE_REASON=all_stages_completed"
echo "DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=${override_state}"
echo "DELIVERY_PIPELINE_ARCHIVE_DIR=${archive_run_dir}"
echo "DELIVERY_PIPELINE_EVIDENCE_JSON=${PIPELINE_EVIDENCE_JSON}"
echo "DELIVERY_PIPELINE_SUMMARY_FILE=${PIPELINE_SUMMARY_FILE}"
