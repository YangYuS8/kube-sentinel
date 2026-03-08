#!/usr/bin/env bash
set -euo pipefail

CRD_SOURCE_DIR="${CRD_SOURCE_DIR:-config/crd}"
CRD_GENERATED_DIR="${CRD_GENERATED_DIR:-.tmp/crd}"
CRD_PATHS="${CRD_PATHS:-./api/v1alpha1}"
CRD_EXPECTED_NAME="${CRD_EXPECTED_NAME:-healingrequests.kubesentinel.io}"
CRD_EXPECTED_GROUP="${CRD_EXPECTED_GROUP:-kubesentinel.io}"
CRD_EXPECTED_VERSION="${CRD_EXPECTED_VERSION:-v1alpha1}"
CRD_CANONICAL_FILE="${CRD_CANONICAL_FILE:-_healingrequests.yaml}"

canonicalize_crd_dir() {
  local crd_dir="$1"
  local grouped_file="${crd_dir}/${CRD_EXPECTED_GROUP}_healingrequests.yaml"
  local canonical_file="${crd_dir}/${CRD_CANONICAL_FILE}"

  if [[ -f "$grouped_file" ]]; then
    cp "$grouped_file" "$canonical_file"
    rm -f "$grouped_file"
  fi
}

validate_crd_dir() {
  local crd_dir="$1"
  local crd_file="${crd_dir}/${CRD_CANONICAL_FILE}"

  if [[ ! -f "$crd_file" ]]; then
    echo "QUALITY_GATE_RESULT=block"
    echo "QUALITY_GATE_CATEGORY=crd_consistency"
    echo "QUALITY_GATE_REASON=crd_manifest_missing"
    echo "QUALITY_GATE_FIX_HINT=run: controller-gen crd paths=${CRD_PATHS} output:crd:artifacts:config=.tmp/crd && cp -r .tmp/crd/* config/crd/"
    return 1
  fi

  if ! grep -Eq "^kind: CustomResourceDefinition$" "$crd_file"; then
    echo "QUALITY_GATE_RESULT=block"
    echo "QUALITY_GATE_CATEGORY=crd_consistency"
    echo "QUALITY_GATE_REASON=crd_manifest_invalid"
    echo "QUALITY_GATE_FIX_HINT=expected kind CustomResourceDefinition in ${crd_file}"
    return 1
  fi

  if ! grep -Eq "^  name: ${CRD_EXPECTED_NAME}$" "$crd_file"; then
    echo "QUALITY_GATE_RESULT=block"
    echo "QUALITY_GATE_CATEGORY=crd_consistency"
    echo "QUALITY_GATE_REASON=crd_manifest_invalid"
    echo "QUALITY_GATE_FIX_HINT=expected metadata.name=${CRD_EXPECTED_NAME} in ${crd_file}"
    return 1
  fi

  if ! grep -Eq "^  group: ${CRD_EXPECTED_GROUP}$" "$crd_file"; then
    echo "QUALITY_GATE_RESULT=block"
    echo "QUALITY_GATE_CATEGORY=crd_consistency"
    echo "QUALITY_GATE_REASON=crd_manifest_invalid"
    echo "QUALITY_GATE_FIX_HINT=expected spec.group=${CRD_EXPECTED_GROUP} in ${crd_file}"
    return 1
  fi

  if ! grep -Eq "^- name: ${CRD_EXPECTED_VERSION}$|^  - name: ${CRD_EXPECTED_VERSION}$|^    name: ${CRD_EXPECTED_VERSION}$" "$crd_file"; then
    echo "QUALITY_GATE_RESULT=block"
    echo "QUALITY_GATE_CATEGORY=crd_consistency"
    echo "QUALITY_GATE_REASON=crd_manifest_invalid"
    echo "QUALITY_GATE_FIX_HINT=expected spec.versions[*].name=${CRD_EXPECTED_VERSION} in ${crd_file}"
    return 1
  fi
}

if [[ "${CRD_CHECK_SKIP_GENERATE:-0}" != "1" ]]; then
  rm -rf "$CRD_GENERATED_DIR"
  mkdir -p "$CRD_GENERATED_DIR"
  controller-gen crd paths="${CRD_PATHS}" "output:crd:artifacts:config=${CRD_GENERATED_DIR}"
fi

canonicalize_crd_dir "$CRD_GENERATED_DIR"
canonicalize_crd_dir "$CRD_SOURCE_DIR"
validate_crd_dir "$CRD_GENERATED_DIR"
validate_crd_dir "$CRD_SOURCE_DIR"

if ! diff -ruN "$CRD_SOURCE_DIR" "$CRD_GENERATED_DIR"; then
  echo "QUALITY_GATE_RESULT=block"
  echo "QUALITY_GATE_CATEGORY=crd_consistency"
  echo "QUALITY_GATE_REASON=crd_generation_drift"
  echo "QUALITY_GATE_FIX_HINT=run: controller-gen crd paths=${CRD_PATHS} output:crd:artifacts:config=.tmp/crd && cp -r .tmp/crd/* config/crd/"
  exit 1
fi

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=crd_consistency"
echo "QUALITY_GATE_REASON=ok"
