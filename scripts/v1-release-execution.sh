#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

STAGE="${V1_RELEASE_STAGE:-rc}"
VERSION_TAG="${V1_RELEASE_VERSION_TAG:-}"
RC_TAG="${V1_RELEASE_RC_TAG:-}"
WORK_DIR="${V1_RELEASE_WORK_DIR:-.tmp/v1-release-execution/${STAGE}-${VERSION_TAG:-unset}}"
TRACE_FILE="${V1_RELEASE_TRACE_FILE:-${WORK_DIR}/release.trace}"
SUMMARY_FILE="${V1_RELEASE_SUMMARY_FILE:-${WORK_DIR}/release-summary.env}"
PLAN_FILE="${V1_RELEASE_PLAN_FILE:-${WORK_DIR}/release-plan.env}"
INSTALL_CMD="${V1_RELEASE_INSTALL_CMD:-bash ${ROOT_DIR}/scripts/install-minimal.sh}"
DEV_CHECK_CMD="${V1_RELEASE_DEV_CHECK_CMD:-bash ${ROOT_DIR}/scripts/dev-local-loop.sh check}"
SMOKE_CMD="${V1_RELEASE_SMOKE_CMD:-bash ${ROOT_DIR}/scripts/drill-runtime-closed-loop.sh default}"
PIPELINE_CMD="${V1_RELEASE_PIPELINE_CMD:-make -C ${ROOT_DIR} delivery-pipeline}"
PREPROD_VERIFIED="${V1_RELEASE_PREPROD_VERIFIED:-false}"
PILOT_VERIFIED="${V1_RELEASE_PILOT_VERIFIED:-false}"
GO_LIVE_VERIFIED="${V1_RELEASE_GO_LIVE_VERIFIED:-false}"
SCOPE_DECLARATION="${V1_RELEASE_SCOPE_DECLARATION:-deployment-only}"

stable_regex='^v[0-9]+\.[0-9]+\.[0-9]+$'
prerelease_regex='^v[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$'

fail() {
  echo "ASSERTION FAILED: $*" >&2
  exit 1
}

info() {
  echo "INFO: $*"
}

extract_key() {
  local file="$1"
  local key="$2"
  local line
  line="$(grep -E "^${key}=" "$file" | tail -n 1 || true)"
  echo "${line#*=}"
}

run_stage() {
  local stage_name="$1"
  local command="$2"
  local output_file="$3"

  echo "$stage_name" >>"$TRACE_FILE"
  info "running ${stage_name}"
  if ! bash -o pipefail -c "$command" >"$output_file" 2>&1; then
    cat "$output_file"
    fail "stage ${stage_name} failed"
  fi
}

validate_scope() {
  [[ "$SCOPE_DECLARATION" == "deployment-only" ]] || fail "v1 release scope must stay deployment-only"
}

validate_version() {
  [[ -n "$VERSION_TAG" ]] || fail "missing V1_RELEASE_VERSION_TAG"
  case "$STAGE" in
    rc)
      [[ "$VERSION_TAG" =~ $prerelease_regex ]] || fail "rc stage requires a prerelease tag such as v1.0.0-rc.1"
      ;;
    stable)
      [[ "$VERSION_TAG" =~ $stable_regex ]] || fail "stable stage requires a stable tag such as v1.0.0"
      [[ "$RC_TAG" =~ $prerelease_regex ]] || fail "stable stage requires V1_RELEASE_RC_TAG to reference a validated prerelease tag"
      [[ "$PREPROD_VERIFIED" == "true" ]] || fail "stable stage requires V1_RELEASE_PREPROD_VERIFIED=true"
      [[ "$PILOT_VERIFIED" == "true" ]] || fail "stable stage requires V1_RELEASE_PILOT_VERIFIED=true"
      [[ "$GO_LIVE_VERIFIED" == "true" ]] || fail "stable stage requires V1_RELEASE_GO_LIVE_VERIFIED=true"
      ;;
    *)
      fail "unsupported V1_RELEASE_STAGE: ${STAGE} (supported: rc, stable)"
      ;;
  esac
}

generate_release_plan() {
  RELEASE_EVENT_NAME=push \
  RELEASE_REF_NAME="$VERSION_TAG" \
  bash "${ROOT_DIR}/scripts/release-image-plan.sh" >"$PLAN_FILE"
}

validate_release_plan() {
  local channel expected_latest actual_latest actual_channel
  actual_channel="$(extract_key "$PLAN_FILE" channel)"
  actual_latest="$(extract_key "$PLAN_FILE" publish_latest)"

  if [[ "$STAGE" == "rc" ]]; then
    channel="prerelease"
    expected_latest="false"
  else
    channel="stable"
    expected_latest="true"
  fi

  [[ "$actual_channel" == "$channel" ]] || fail "release plan channel mismatch: expected ${channel}, got ${actual_channel}"
  [[ "$actual_latest" == "$expected_latest" ]] || fail "release plan latest mismatch: expected ${expected_latest}, got ${actual_latest}"
}

validate_pipeline_output() {
  local pipeline_file="$1"
  local result
  result="$(grep -E '^DELIVERY_PIPELINE_RESULT=' "$pipeline_file" | tail -n 1 | cut -d= -f2- || true)"
  [[ "$result" == "allow" ]] || fail "delivery pipeline must finish with allow, got ${result:-<missing>}"
}

write_summary() {
  local pipeline_log="$1"
  local plan_version plan_channel plan_latest
  plan_version="$(extract_key "$PLAN_FILE" version_tag)"
  plan_channel="$(extract_key "$PLAN_FILE" channel)"
  plan_latest="$(extract_key "$PLAN_FILE" publish_latest)"

  cat >"$SUMMARY_FILE" <<EOF
V1_RELEASE_SEQUENCE_RESULT=pass
V1_RELEASE_STAGE=${STAGE}
V1_RELEASE_VERSION_TAG=${VERSION_TAG}
V1_RELEASE_RC_TAG=${RC_TAG}
V1_RELEASE_SCOPE_DECLARATION=${SCOPE_DECLARATION}
V1_RELEASE_EXCLUDED_SCOPE=statefulset-write-path,extra-ui-query-layer
V1_RELEASE_EVIDENCE_DIR=${WORK_DIR}
V1_RELEASE_TRACE_FILE=${TRACE_FILE}
V1_RELEASE_SUMMARY_FILE=${SUMMARY_FILE}
V1_RELEASE_PLAN_FILE=${PLAN_FILE}
V1_RELEASE_PIPELINE_LOG=${pipeline_log}
V1_RELEASE_PLAN_VERSION_TAG=${plan_version}
V1_RELEASE_PLAN_CHANNEL=${plan_channel}
V1_RELEASE_PLAN_PUBLISH_LATEST=${plan_latest}
V1_RELEASE_PREPROD_VERIFIED=${PREPROD_VERIFIED}
V1_RELEASE_PILOT_VERIFIED=${PILOT_VERIFIED}
V1_RELEASE_GO_LIVE_VERIFIED=${GO_LIVE_VERIFIED}
EOF
}

main() {
  mkdir -p "$WORK_DIR"
  : >"$TRACE_FILE"

  validate_scope
  validate_version

  local install_log="${WORK_DIR}/01-install.log"
  local dev_log="${WORK_DIR}/02-dev-check.log"
  local smoke_log="${WORK_DIR}/03-smoke.log"
  local pipeline_log="${WORK_DIR}/04-delivery-pipeline.log"

  if [[ "$STAGE" == "rc" ]]; then
    run_stage "install_minimal" "$INSTALL_CMD" "$install_log"
    run_stage "dev_local_loop_check" "$DEV_CHECK_CMD" "$dev_log"
    run_stage "runtime_closed_loop_smoke" "$SMOKE_CMD" "$smoke_log"
  fi

  run_stage "delivery_pipeline" "$PIPELINE_CMD" "$pipeline_log"
  validate_pipeline_output "$pipeline_log"

  run_stage "release_image_plan" "RELEASE_EVENT_NAME=push RELEASE_REF_NAME=${VERSION_TAG} bash ${ROOT_DIR}/scripts/release-image-plan.sh" "$PLAN_FILE"
  validate_release_plan
  write_summary "$pipeline_log"

  cat "$SUMMARY_FILE"
}

main "$@"