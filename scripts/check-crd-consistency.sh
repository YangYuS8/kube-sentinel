#!/usr/bin/env bash
set -euo pipefail

CRD_SOURCE_DIR="${CRD_SOURCE_DIR:-config/crd}"
CRD_GENERATED_DIR="${CRD_GENERATED_DIR:-.tmp/crd}"

if [[ "${CRD_CHECK_SKIP_GENERATE:-0}" != "1" ]]; then
  mkdir -p "$CRD_GENERATED_DIR"
  controller-gen crd paths=./api/... "output:crd:artifacts:config=${CRD_GENERATED_DIR}"
fi

if ! diff -ruN "$CRD_SOURCE_DIR" "$CRD_GENERATED_DIR"; then
  echo "QUALITY_GATE_RESULT=block"
  echo "QUALITY_GATE_CATEGORY=crd_consistency"
  echo "QUALITY_GATE_REASON=crd_generation_drift"
  echo "QUALITY_GATE_FIX_HINT=run: controller-gen crd paths=./api/... output:crd:artifacts:config=.tmp/crd && cp -r .tmp/crd/* config/crd/"
  exit 1
fi

echo "QUALITY_GATE_RESULT=allow"
echo "QUALITY_GATE_CATEGORY=crd_consistency"
echo "QUALITY_GATE_REASON=ok"
