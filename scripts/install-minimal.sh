#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
NAMESPACE="${KUBE_SENTINEL_NAMESPACE:-kube-sentinel-system}"
IMAGE="${KUBE_SENTINEL_IMAGE:-kube-sentinel/controller:latest}"
BUILD_IMAGE="${KUBE_SENTINEL_BUILD_IMAGE:-true}"
IMAGE_BUILDER="${KUBE_SENTINEL_IMAGE_BUILDER:-}"
DRY_RUN="${KUBE_SENTINEL_INSTALL_DRY_RUN:-false}"
RUNTIME_MODE="${KUBE_SENTINEL_RUNTIME_MODE:-minimal}"
READ_ONLY_MODE="${KUBE_SENTINEL_READ_ONLY_MODE:-false}"
MANIFEST_TEMPLATE="${ROOT_DIR}/config/install/kube-sentinel.yaml"
CRD_FILE="${ROOT_DIR}/config/crd/_healingrequests.yaml"

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

bool_is_true() {
  [[ "${1:-false}" == "true" ]]
}

select_image_builder() {
  if [[ -n "$IMAGE_BUILDER" ]]; then
    require_binary "$IMAGE_BUILDER"
    echo "$IMAGE_BUILDER"
    return
  fi
  if command -v docker >/dev/null 2>&1; then
    echo docker
    return
  fi
  if command -v podman >/dev/null 2>&1; then
    echo podman
    return
  fi
  fail "missing docker or podman; set KUBE_SENTINEL_BUILD_IMAGE=false to skip image build"
}

resolve_built_image() {
  local builder="$1"
  local image="$2"
  if [[ "$image" != localhost/* ]] && "$builder" images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -Fx "localhost/$image" >/dev/null 2>&1; then
    echo "localhost/$image"
    return
  fi
  if "$builder" images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -Fx "$image" >/dev/null 2>&1; then
    echo "$image"
    return
  fi
  if "$builder" image inspect "$image" >/dev/null 2>&1; then
    echo "$image"
    return
  fi
  if [[ "$image" != localhost/* ]] && "$builder" image inspect "localhost/$image" >/dev/null 2>&1; then
    echo "localhost/$image"
    return
  fi
  echo "$image"
}

current_context() {
  kubectl config current-context 2>/dev/null || true
}

render_manifest() {
  sed \
    -e "s|__KUBE_SENTINEL_NAMESPACE__|${NAMESPACE}|g" \
    -e "s|__KUBE_SENTINEL_IMAGE__|${IMAGE}|g" \
    -e "s|__KUBE_SENTINEL_RUNTIME_MODE__|${RUNTIME_MODE}|g" \
    -e "s|__KUBE_SENTINEL_READ_ONLY_MODE__|${READ_ONLY_MODE}|g" \
    "$MANIFEST_TEMPLATE"
}

wait_for_controller_rollout() {
  local timeout="${KUBE_SENTINEL_ROLLOUT_TIMEOUT:-120s}"
  if kubectl -n "$NAMESPACE" rollout status deployment/kube-sentinel --timeout="$timeout" >/dev/null 2>&1; then
    info "controller rollout is ready"
    return
  fi
  info "controller rollout failed; current pod status:"
  kubectl -n "$NAMESPACE" get pods -o wide || true
  fail "controller rollout failed; if you are using minikube, check proxy/registry access or provide a prebuilt image via KUBE_SENTINEL_IMAGE and KUBE_SENTINEL_BUILD_IMAGE=false"
}

controller_deployment_exists() {
  kubectl -n "$NAMESPACE" get deployment kube-sentinel >/dev/null 2>&1
}

build_image_if_needed() {
  if ! bool_is_true "$BUILD_IMAGE"; then
    info "skip image build (KUBE_SENTINEL_BUILD_IMAGE=${BUILD_IMAGE})"
    return
  fi

  local builder
  builder="$(select_image_builder)"

  local context
  context="$(current_context)"
  info "building controller image with ${builder}: ${IMAGE}"
  "$builder" build -t "$IMAGE" "$ROOT_DIR"
  IMAGE="$(resolve_built_image "$builder" "$IMAGE")"
  info "using local image reference ${IMAGE}"

  if [[ "$context" == minikube* ]] && command -v minikube >/dev/null 2>&1; then
    local archive
    archive="$(mktemp /tmp/kube-sentinel-image.XXXXXX.tar)"
    info "saving local image archive for minikube: ${archive}"
    "$builder" save -o "$archive" "$IMAGE"
    info "loading image archive into minikube profile ${context}"
    minikube image load "$archive" >/dev/null
    rm -f "$archive"
  fi
}

main() {
  require_binary kubectl

  if ! kubectl cluster-info >/dev/null 2>&1; then
    fail "kubectl 当前无法访问集群"
  fi

  if [[ -z "$(current_context)" ]]; then
    fail "kubectl current-context 为空，请先选择可用集群"
  fi

  if [[ ! -f "$MANIFEST_TEMPLATE" ]]; then
    fail "missing install manifest template: ${MANIFEST_TEMPLATE}"
  fi
  if [[ ! -f "$CRD_FILE" ]]; then
    fail "missing CRD file: ${CRD_FILE}"
  fi

  build_image_if_needed

  info "ensuring namespace ${NAMESPACE}"
  kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

  info "installing HealingRequest CRD"
  kubectl apply -f "$CRD_FILE" >/dev/null

  info "applying kube-sentinel controller resources"
  if bool_is_true "$DRY_RUN"; then
    render_manifest
  else
    local existing_deployment="false"
    if controller_deployment_exists; then
      existing_deployment="true"
    fi
    render_manifest | kubectl apply -f - >/dev/null
    if [[ "$existing_deployment" == "true" ]]; then
      info "restarting existing kube-sentinel deployment to pick up refreshed image content"
      kubectl -n "$NAMESPACE" rollout restart deployment/kube-sentinel >/dev/null
    fi
    wait_for_controller_rollout
  fi

  info "next steps:"
  echo "  1. kubectl -n ${NAMESPACE} rollout status deployment/kube-sentinel"
  echo "  2. 在另一个终端运行 bash ./scripts/dev-local-loop.sh connect-cluster"
  echo "  3. 执行 bash ./scripts/drill-runtime-closed-loop.sh default"
  echo "  4. 只读模式示例: KUBE_SENTINEL_READ_ONLY_MODE=true bash ./scripts/install-minimal.sh"
  if ! bool_is_true "$BUILD_IMAGE"; then
    echo "  NOTE: 你跳过了镜像构建；若 Pod ImagePullBackOff，请先执行 docker/podman build，并在 minikube 中执行 minikube image load ${IMAGE}"
  fi
}

main "$@"
