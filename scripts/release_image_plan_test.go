package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestReleaseImagePlanStableTagFromPushPublishesLatest(t *testing.T) {
	env := map[string]string{
		"RELEASE_EVENT_NAME": "push",
		"RELEASE_REF_NAME":   "v1.2.3",
	}
	output, err := runShellScript(t, "./release-image-plan.sh", nil, env)
	if err != nil {
		t.Fatalf("expected stable push tag to succeed, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"image_name=ghcr.io/yangyus8/kube-sentinel",
		"version_tag=v1.2.3",
		"channel=stable",
		"publish_latest=true",
		"tags=ghcr.io/yangyus8/kube-sentinel:v1.2.3,ghcr.io/yangyus8/kube-sentinel:latest",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}

func TestReleaseImagePlanPrereleaseTagFromPushSkipsLatest(t *testing.T) {
	env := map[string]string{
		"RELEASE_EVENT_NAME": "push",
		"RELEASE_REF_NAME":   "v1.2.3-rc.1",
	}
	output, err := runShellScript(t, "./release-image-plan.sh", nil, env)
	if err != nil {
		t.Fatalf("expected prerelease push tag to succeed, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"version_tag=v1.2.3-rc.1",
		"channel=prerelease",
		"is_prerelease=true",
		"publish_latest=false",
		"tags=ghcr.io/yangyus8/kube-sentinel:v1.2.3-rc.1",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
	if strings.Contains(output, ":latest") {
		t.Fatalf("prerelease output should not contain latest tag: %s", output)
	}
}

func TestReleaseImagePlanRejectsInvalidPushTag(t *testing.T) {
	env := map[string]string{
		"RELEASE_EVENT_NAME": "push",
		"RELEASE_REF_NAME":   "vnext",
	}
	output, err := runShellScript(t, "./release-image-plan.sh", nil, env)
	if err == nil {
		t.Fatalf("expected invalid push tag to fail, output: %s", output)
	}
	if !strings.Contains(output, "requires a v-prefixed semantic version tag") {
		t.Fatalf("expected semantic version validation failure, output: %s", output)
	}
}

func TestReleaseImagePlanWorkflowDispatchUsesManualTagWithoutLatest(t *testing.T) {
	env := map[string]string{
		"RELEASE_EVENT_NAME": "workflow_dispatch",
		"RELEASE_MANUAL_TAG": "manual-preview-123",
	}
	output, err := runShellScript(t, "./release-image-plan.sh", nil, env)
	if err != nil {
		t.Fatalf("expected workflow_dispatch plan to succeed, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"version_tag=manual-preview-123",
		"channel=manual",
		"publish_latest=false",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}

func TestVerifyReleaseManifestSucceedsForDualArchImage(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "docker", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "buildx" && "$2" == "imagetools" && "$3" == "inspect" ]]; then
	cat <<EOF
Name: ghcr.io/yangyus8/kube-sentinel:v1.2.3
MediaType: application/vnd.oci.image.index.v1+json
Manifests:
	Name: ghcr.io/yangyus8/kube-sentinel:v1.2.3@sha256:amd64
	Platform: linux/amd64
	Name: ghcr.io/yangyus8/kube-sentinel:v1.2.3@sha256:arm64
	Platform: linux/arm64
EOF
	exit 0
fi
exit 1
`)

	env := map[string]string{
		"PATH":               binDir + ":" + os.Getenv("PATH"),
		"RELEASE_IMAGE_NAME": "ghcr.io/yangyus8/kube-sentinel",
		"RELEASE_TAG":        "v1.2.3",
	}
	output, err := runShellScript(t, "./verify-release-manifest.sh", nil, env)
	if err != nil {
		t.Fatalf("expected manifest verification to pass, got %v, output: %s", err, output)
	}
	if !strings.Contains(output, "RELEASE_MANIFEST_RESULT=pass") {
		t.Fatalf("expected pass output, got: %s", output)
	}
}

func TestVerifyReleaseManifestFailsWhenArm64Missing(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "docker", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "buildx" && "$2" == "imagetools" && "$3" == "inspect" ]]; then
	cat <<EOF
Name: ghcr.io/yangyus8/kube-sentinel:v1.2.3
MediaType: application/vnd.oci.image.index.v1+json
Manifests:
	Name: ghcr.io/yangyus8/kube-sentinel:v1.2.3@sha256:amd64
	Platform: linux/amd64
EOF
	exit 0
fi
exit 1
`)

	env := map[string]string{
		"PATH":               binDir + ":" + os.Getenv("PATH"),
		"RELEASE_IMAGE_NAME": "ghcr.io/yangyus8/kube-sentinel",
		"RELEASE_TAG":        "v1.2.3",
	}
	output, err := runShellScript(t, "./verify-release-manifest.sh", nil, env)
	if err == nil {
		t.Fatalf("expected manifest verification to fail, output: %s", output)
	}
	if !strings.Contains(output, "missing platform linux/arm64") {
		t.Fatalf("expected arm64 failure, output: %s", output)
	}
}
