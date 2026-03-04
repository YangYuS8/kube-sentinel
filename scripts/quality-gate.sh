#!/usr/bin/env bash
set -euo pipefail

TRACE_FILE="${QUALITY_GATE_TRACE_FILE:-}"

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
  echo "QUALITY_GATE_RESULT=block"
  echo "QUALITY_GATE_CATEGORY=${category}"
  echo "QUALITY_GATE_REASON=${reason}"
  echo "QUALITY_GATE_FIX_HINT=${fix_hint}"
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
    print_failure "$category" "$reason" "$fix_hint"
    return 1
  fi
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

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=quality_gate"
echo "QUALITY_GATE_REASON=all_checks_passed"
