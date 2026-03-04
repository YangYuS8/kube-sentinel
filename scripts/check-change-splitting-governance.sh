#!/usr/bin/env bash
set -euo pipefail

is_int() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

split_contains() {
  local value="$1"
  local token="$2"
  case ",${value}," in
    *",${token},"*) return 0 ;;
    *) return 1 ;;
  esac
}

normalize_bool() {
  local value="${1:-false}"
  case "$value" in
    true|false) echo "$value" ;;
    *) echo "false" ;;
  esac
}

print_block() {
  local reason="$1"
  local fix_hint="$2"
  echo "QUALITY_GATE_RESULT=block"
  echo "QUALITY_GATE_CATEGORY=change_splitting_governance"
  echo "QUALITY_GATE_REASON=${reason}"
  echo "QUALITY_GATE_FIX_HINT=${fix_hint}"
}

GOV_ENABLED="$(normalize_bool "${CHANGE_SPLIT_GOVERNANCE_ENABLED:-false}")"
if [[ "$GOV_ENABLED" != "true" ]]; then
  echo "QUALITY_GATE_RESULT=allow"
  echo "QUALITY_GATE_CATEGORY=change_splitting_governance"
  echo "QUALITY_GATE_REASON=governance_check_disabled"
  echo "QUALITY_GATE_FIX_HINT=n/a"
  exit 0
fi

STAGE="${CHANGE_SPLIT_STAGE:-proposal}"
CAPABILITY_COUNT="${CHANGE_SPLIT_CAPABILITY_COUNT:-0}"
INCREMENT_ITEMS="${CHANGE_SPLIT_INCREMENT_ITEMS:-0}"
HAS_SPLIT_PLAN="$(normalize_bool "${CHANGE_SPLIT_HAS_SPLIT_PLAN:-false}")"
SPLIT_PLAN_REF="${CHANGE_SPLIT_PLAN_REF:-}"
RISK_DOMAINS="${CHANGE_SPLIT_RISK_DOMAINS:-}"

EXCEPTION_APPROVED="$(normalize_bool "${CHANGE_SPLIT_EXCEPTION_APPROVED:-false}")"
EXCEPTION_ACTOR="${CHANGE_SPLIT_EXCEPTION_APPROVER:-}"
EXCEPTION_REASON="${CHANGE_SPLIT_EXCEPTION_REASON:-}"
EXCEPTION_TIMESTAMP="${CHANGE_SPLIT_EXCEPTION_TIMESTAMP:-}"
EXCEPTION_TRACE_KEY="${CHANGE_SPLIT_EXCEPTION_TRACE_KEY:-}"

CHECK_SCOPE_COMPLEXITY="${CHANGE_SPLIT_CHECK_SCOPE_COMPLEXITY:-}"
CHECK_RISK_COUPLING="${CHANGE_SPLIT_CHECK_RISK_COUPLING:-}"
CHECK_REVIEWABILITY="${CHANGE_SPLIT_CHECK_REVIEWABILITY:-}"
CHECK_ROLLBACK_IMPACT="${CHANGE_SPLIT_CHECK_ROLLBACK_IMPACT:-}"

IDEMPOTENCY_FILE="${CHANGE_SPLIT_IDEMPOTENCY_FILE:-}"
SUBMISSION_KEY="${CHANGE_SPLIT_SUBMISSION_KEY:-}"

if ! is_int "$CAPABILITY_COUNT"; then
  print_block "capability_count_invalid" "set CHANGE_SPLIT_CAPABILITY_COUNT to integer"
  exit 1
fi
if ! is_int "$INCREMENT_ITEMS"; then
  print_block "increment_items_invalid" "set CHANGE_SPLIT_INCREMENT_ITEMS to integer"
  exit 1
fi

if [[ -n "$IDEMPOTENCY_FILE" && -n "$SUBMISSION_KEY" ]]; then
  mkdir -p "$(dirname "$IDEMPOTENCY_FILE")"
  touch "$IDEMPOTENCY_FILE"
  if grep -Fxq "$SUBMISSION_KEY" "$IDEMPOTENCY_FILE"; then
    echo "QUALITY_GATE_RESULT=allow"
    echo "QUALITY_GATE_CATEGORY=change_splitting_governance"
    echo "QUALITY_GATE_REASON=idempotent_replay"
    echo "QUALITY_GATE_FIX_HINT=n/a"
    echo "QUALITY_GATE_GOV_IDEMPOTENT=true"
    exit 0
  fi
fi

needs_split="false"
if (( CAPABILITY_COUNT >= 3 )); then
  needs_split="true"
fi
if (( INCREMENT_ITEMS > 10 )); then
  needs_split="true"
fi

has_blocking="false"
has_operational="false"
if split_contains "$RISK_DOMAINS" "blocking"; then
  has_blocking="true"
fi
if split_contains "$RISK_DOMAINS" "operational"; then
  has_operational="true"
fi

if [[ "$has_blocking" == "true" && "$has_operational" == "true" ]]; then
  if [[ "$EXCEPTION_APPROVED" != "true" ]]; then
    print_block "mixed_risk_domains_without_exception" "split change into blocking and operational tracks or provide exception approval"
    exit 1
  fi
fi

if [[ "$EXCEPTION_APPROVED" == "true" ]]; then
  if [[ -z "$EXCEPTION_ACTOR" || -z "$EXCEPTION_REASON" || -z "$EXCEPTION_TIMESTAMP" || -z "$EXCEPTION_TRACE_KEY" ]]; then
    print_block "exception_approval_fields_missing" "set approver/reason/timestamp/trace key for approved exception"
    exit 1
  fi
fi

if [[ "$STAGE" == "archive" ]]; then
  if [[ -z "$CHECK_SCOPE_COMPLEXITY" || -z "$CHECK_RISK_COUPLING" || -z "$CHECK_REVIEWABILITY" || -z "$CHECK_ROLLBACK_IMPACT" ]]; then
    print_block "pre_archive_checklist_incomplete" "fill scope_complexity/risk_coupling/reviewability/rollback_impact"
    exit 1
  fi
fi

if [[ "$needs_split" == "true" && "$HAS_SPLIT_PLAN" != "true" && "$EXCEPTION_APPROVED" != "true" ]]; then
  print_block "split_required_missing_plan" "provide split plan (CHANGE_SPLIT_HAS_SPLIT_PLAN=true + CHANGE_SPLIT_PLAN_REF) or approved exception"
  exit 1
fi

if [[ "$HAS_SPLIT_PLAN" == "true" && -z "$SPLIT_PLAN_REF" ]]; then
  print_block "split_plan_ref_missing" "set CHANGE_SPLIT_PLAN_REF when CHANGE_SPLIT_HAS_SPLIT_PLAN=true"
  exit 1
fi

if [[ -n "$IDEMPOTENCY_FILE" && -n "$SUBMISSION_KEY" ]]; then
  echo "$SUBMISSION_KEY" >>"$IDEMPOTENCY_FILE"
fi

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=change_splitting_governance"
if [[ "$needs_split" == "true" ]]; then
  if [[ "$HAS_SPLIT_PLAN" == "true" ]]; then
    echo "QUALITY_GATE_REASON=split_plan_present"
    echo "QUALITY_GATE_FIX_HINT=n/a"
    echo "QUALITY_GATE_SPLIT_PLAN_REF=${SPLIT_PLAN_REF}"
  else
    echo "QUALITY_GATE_REASON=exception_approved"
    echo "QUALITY_GATE_FIX_HINT=n/a"
    echo "QUALITY_GATE_EXCEPTION_APPROVER=${EXCEPTION_ACTOR}"
    echo "QUALITY_GATE_EXCEPTION_REASON=${EXCEPTION_REASON}"
    echo "QUALITY_GATE_EXCEPTION_TIMESTAMP=${EXCEPTION_TIMESTAMP}"
    echo "QUALITY_GATE_EXCEPTION_TRACE_KEY=${EXCEPTION_TRACE_KEY}"
  fi
else
  echo "QUALITY_GATE_REASON=below_split_threshold"
  echo "QUALITY_GATE_FIX_HINT=n/a"
fi
