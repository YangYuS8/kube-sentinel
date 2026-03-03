#!/usr/bin/env bash
set -euo pipefail

mkdir -p .tmp/crd
controller-gen crd paths=./api/... output:crd:artifacts:config=.tmp/crd
if ! diff -ruN config/crd .tmp/crd; then
  echo "CRD generation drift detected"
  exit 1
fi
