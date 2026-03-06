#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODE="${1:-run-local}"
SYSTEM_NAMESPACE="${KUBE_SENTINEL_NAMESPACE:-kube-sentinel-system}"
DEMO_NAMESPACE="${KUBE_SENTINEL_DEMO_NAMESPACE:-default}"
DRY_RUN="${KUBE_SENTINEL_DEV_LOOP_DRY_RUN:-false}"

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

current_context() {
  kubectl config current-context 2>/dev/null || true
}

ensure_cluster_ready() {
  local context
  context="$(current_context)"
  if [[ -z "$context" ]]; then
    fail "kubectl current-context 为空，请先执行 kubectl config use-context <context>"
  fi
  info "using kube context: ${context}"

  if ! kubectl cluster-info >/dev/null 2>&1; then
    fail "kubectl 当前无法访问集群"
  fi

  local kube_system_pods
  kube_system_pods="$(kubectl -n kube-system get pods --no-headers 2>/dev/null || true)"
  if [[ -z "$kube_system_pods" ]]; then
    fail "kube-system 没有可见 Pod，基础组件尚未就绪"
  fi
  if ! grep -q "coredns" <<<"$kube_system_pods"; then
    fail "kube-system 缺少 coredns，请先等待 DNS 组件就绪"
  fi
  if grep "coredns" <<<"$kube_system_pods" | grep -vq "Running"; then
    fail "coredns 尚未 Running，请先等待基础组件就绪"
  fi
  if grep -E "storage-provisioner|csi-hostpath-provisioner|local-path-provisioner" <<<"$kube_system_pods" >/dev/null 2>&1; then
    if grep -E "storage-provisioner|csi-hostpath-provisioner|local-path-provisioner" <<<"$kube_system_pods" | grep -vq "Running"; then
      fail "存储 provisioner 尚未 Running，请先等待基础组件就绪"
    fi
  else
    fail "kube-system 缺少常见 storage provisioner，请先确认 minikube 基础组件是否完成启动"
  fi
}

ensure_crd() {
  if kubectl get crd healingrequests.kubesentinel.io >/dev/null 2>&1; then
    return
  fi
  info "installing HealingRequest CRD"
  kubectl apply -f "$ROOT_DIR/config/crd/_healingrequests.yaml" >/dev/null
}

ensure_demo_workload() {
  if kubectl -n "$DEMO_NAMESPACE" get deployment demo-app >/dev/null 2>&1; then
    info "demo workload already present: ${DEMO_NAMESPACE}/demo-app"
    return
  fi
  info "creating demo deployment ${DEMO_NAMESPACE}/demo-app"
  cat <<EOF | kubectl apply -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo-app
  namespace: ${DEMO_NAMESPACE}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: demo-app
  template:
    metadata:
      labels:
        app: demo-app
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
  name: demo-app
  namespace: ${DEMO_NAMESPACE}
spec:
  selector:
    app: demo-app
  ports:
    - port: 80
      targetPort: 80
EOF
}

assert_port_free() {
  local port="$1"
  if (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1; then
    fail "port ${port} 已被占用，请先释放后再本地启动 manager"
  fi
}

run_local_manager() {
  require_binary go
  assert_port_free 8080
  assert_port_free 8081
  assert_port_free 8090
  if bool_is_true "$DRY_RUN"; then
    info "dry-run: local manager checks passed"
    return
  fi
  cd "$ROOT_DIR"
  exec go run ./cmd/manager
}

connect_cluster_manager() {
  if ! kubectl -n "$SYSTEM_NAMESPACE" get svc kube-sentinel >/dev/null 2>&1; then
    fail "missing service ${SYSTEM_NAMESPACE}/kube-sentinel，请先运行 bash ./scripts/install-minimal.sh"
  fi
  if bool_is_true "$DRY_RUN"; then
    info "dry-run: cluster manager connection checks passed"
    return
  fi
  exec kubectl -n "$SYSTEM_NAMESPACE" port-forward svc/kube-sentinel 8090:8090
}

print_next_steps() {
  echo "INFO: next steps"
  echo "  1. 本地运行 manager: bash ./scripts/dev-local-loop.sh run-local"
  echo "  2. 或连接集群 manager: bash ./scripts/dev-local-loop.sh connect-cluster"
  echo "  3. 触发 smoke: bash ./scripts/drill-runtime-closed-loop.sh ${DEMO_NAMESPACE}"
}

main() {
  require_binary kubectl
  require_binary curl
  ensure_cluster_ready
  ensure_crd
  ensure_demo_workload

  case "$MODE" in
    check)
      print_next_steps
      ;;
    run-local)
      run_local_manager
      ;;
    connect-cluster)
      connect_cluster_manager
      ;;
    *)
      fail "unsupported mode: ${MODE} (supported: check, run-local, connect-cluster)"
      ;;
  esac
}

main "$@"