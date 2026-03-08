#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${RELEASE_IMAGE_NAME:-}"
TAG_NAME="${RELEASE_TAG:-}"
IMAGE_REF="${RELEASE_IMAGE_REF:-}"

fail() {
  echo "ASSERTION FAILED: $*" >&2
  exit 1
}

if [[ -z "$IMAGE_REF" ]]; then
  [[ -n "$IMAGE_NAME" ]] || fail "missing RELEASE_IMAGE_NAME"
  [[ -n "$TAG_NAME" ]] || fail "missing RELEASE_TAG"
  IMAGE_REF="${IMAGE_NAME}:${TAG_NAME}"
fi

if ! command -v docker >/dev/null 2>&1; then
  fail "missing required binary: docker"
fi

inspect_output="$(docker buildx imagetools inspect "$IMAGE_REF")"

for platform in linux/amd64 linux/arm64; do
  if ! grep -F "$platform" >/dev/null <<<"$inspect_output"; then
    fail "manifest ${IMAGE_REF} missing platform ${platform}"
  fi
done

echo "RELEASE_MANIFEST_RESULT=pass"
echo "RELEASE_MANIFEST_IMAGE_REF=${IMAGE_REF}"
echo "RELEASE_MANIFEST_PLATFORMS=linux/amd64,linux/arm64"