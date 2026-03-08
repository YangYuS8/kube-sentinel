#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${1:-default}"
TEST_PATTERN="${KUBE_SENTINEL_STATEFULSET_REALITY_TEST_PATTERN:-TestMinikubeStatefulSet}"
ENABLED="${KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY:-false}"
ALLOW_NON_MINIKUBE="${KUBE_SENTINEL_STATEFULSET_REALITY_ALLOW_NON_MINIKUBE:-false}"

emit_skip() {
  echo "STATEFULSET_REALITY_NAMESPACE=${NAMESPACE}"
  echo "STATEFULSET_REALITY_TEST_PATTERN=${TEST_PATTERN}"
  echo "STATEFULSET_REALITY_RESULT=skip"
  echo "STATEFULSET_REALITY_REASON=$1"
  exit 0
}

require_binary() {
  local name="$1"
  if ! command -v "$name" >/dev/null 2>&1; then
    emit_skip "missing_binary_${name}"
  fi
}

if [[ "$ENABLED" != "true" ]]; then
  emit_skip "set_KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY_true"
fi

require_binary kubectl
require_binary go

if ! kubectl cluster-info >/dev/null 2>&1; then
  emit_skip "cluster_unreachable"
fi

CONTEXT="$(kubectl config current-context 2>/dev/null || true)"
if [[ -z "$CONTEXT" ]]; then
  emit_skip "missing_kube_context"
fi
if [[ "$CONTEXT" != "minikube" && "$ALLOW_NON_MINIKUBE" != "true" ]]; then
  emit_skip "context_${CONTEXT}_is_not_minikube"
fi

echo "STATEFULSET_REALITY_NAMESPACE=${NAMESPACE}"
echo "STATEFULSET_REALITY_CONTEXT=${CONTEXT}"
echo "STATEFULSET_REALITY_TEST_PATTERN=${TEST_PATTERN}"

if go test ./internal/healing -run "${TEST_PATTERN}" -count=1 -v; then
  echo "STATEFULSET_REALITY_RESULT=pass"
else
  echo "STATEFULSET_REALITY_RESULT=fail"
  echo "STATEFULSET_REALITY_REASON=go_test_failed"
  exit 1
fi