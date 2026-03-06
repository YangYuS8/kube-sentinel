package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestInstallMinimalDryRunRendersManifestAndNextSteps(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "config" && "$2" == "current-context" ]]; then
	echo minikube
	exit 0
fi
if [[ "$1" == "cluster-info" ]]; then
	exit 0
fi
if [[ "$1" == "create" && "$2" == "namespace" ]]; then
	cat <<EOF
apiVersion: v1
kind: Namespace
metadata:
	name: $3
EOF
	exit 0
fi
if [[ "$1" == "apply" && "$2" == "-f" ]]; then
	cat >/dev/null || true
	exit 0
fi
exit 0
`)

	env := map[string]string{
		"PATH":                          binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_BUILD_IMAGE":     "false",
		"KUBE_SENTINEL_INSTALL_DRY_RUN": "true",
		"KUBE_SENTINEL_NAMESPACE":       "test-system",
		"KUBE_SENTINEL_IMAGE":           "example.local/kube-sentinel:test",
	}
	output, err := runShellScript(t, "./install-minimal.sh", nil, env)
	if err != nil {
		t.Fatalf("expected install dry-run to pass, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"namespace: test-system",
		"image: example.local/kube-sentinel:test",
		"kubectl -n test-system rollout status deployment/kube-sentinel",
		"bash ./scripts/drill-runtime-closed-loop.sh default",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}
