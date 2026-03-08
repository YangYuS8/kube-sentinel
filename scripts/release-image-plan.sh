#!/usr/bin/env bash
set -euo pipefail

IMAGE_NAME="${RELEASE_IMAGE_NAME:-ghcr.io/yangyus8/kube-sentinel}"
EVENT_NAME="${RELEASE_EVENT_NAME:-${GITHUB_EVENT_NAME:-}}"
REF_NAME="${RELEASE_REF_NAME:-${GITHUB_REF_NAME:-}}"
MANUAL_TAG="${RELEASE_MANUAL_TAG:-}"
SHA_VALUE="${RELEASE_SHA:-${GITHUB_SHA:-}}"

fail() {
  echo "ASSERTION FAILED: $*" >&2
  exit 1
}

semver_regex='^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'
manual_regex='^manual-[a-z0-9][a-z0-9._-]*$'

version_tag=""

case "$EVENT_NAME" in
  push)
    [[ -n "$REF_NAME" ]] || fail "missing RELEASE_REF_NAME for push event"
    [[ "$REF_NAME" =~ $semver_regex ]] || fail "push event requires a v-prefixed semantic version tag"
    version_tag="$REF_NAME"
    ;;
  workflow_dispatch)
    if [[ -n "$MANUAL_TAG" ]]; then
      version_tag="$MANUAL_TAG"
    elif [[ -n "$SHA_VALUE" ]]; then
      version_tag="manual-${SHA_VALUE:0:12}"
    else
      fail "workflow_dispatch requires RELEASE_MANUAL_TAG or RELEASE_SHA"
    fi
    if [[ ! "$version_tag" =~ $semver_regex ]] && [[ ! "$version_tag" =~ $manual_regex ]]; then
      fail "workflow_dispatch tag must be a semantic version or manual-* tag"
    fi
    ;;
  *)
    fail "unsupported release event: $EVENT_NAME"
    ;;
esac

is_prerelease=false
publish_latest=false
channel="manual"

if [[ "$version_tag" =~ $semver_regex ]]; then
  if [[ "$version_tag" == *-* ]]; then
    is_prerelease=true
    channel="prerelease"
  else
    channel="stable"
  fi
fi

if [[ "$EVENT_NAME" == "push" && "$channel" == "stable" ]]; then
  publish_latest=true
fi

tags="${IMAGE_NAME}:${version_tag}"
if [[ "$publish_latest" == "true" ]]; then
  tags="${tags},${IMAGE_NAME}:latest"
fi

cat <<EOF
image_name=${IMAGE_NAME}
version_tag=${version_tag}
tags=${tags}
channel=${channel}
is_prerelease=${is_prerelease}
publish_latest=${publish_latest}
EOF