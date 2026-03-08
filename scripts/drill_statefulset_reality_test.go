package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestStatefulSetRealityDrillSkipsWhenDisabled(t *testing.T) {
	output, err := runShellScript(t, "./drill-statefulset-reality.sh", []string{"default"}, nil)
	if err != nil {
		t.Fatalf("expected disabled reality drill to skip cleanly, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"STATEFULSET_REALITY_NAMESPACE=default",
		"STATEFULSET_REALITY_RESULT=skip",
		"STATEFULSET_REALITY_REASON=set_KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY_true",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}

func TestStatefulSetRealityDrillRunsWhenEnabled(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "cluster-info" ]]; then
	exit 0
fi
if [[ "$1" == "config" && "$2" == "current-context" ]]; then
	echo minikube
	exit 0
fi
exit 0
`)
	writeExecutable(t, binDir, "go", `#!/usr/bin/env bash
set -euo pipefail
echo "GO_TEST_ARGS=$*"
exit 0
`)
	env := map[string]string{
		"PATH": binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY":     "true",
		"KUBE_SENTINEL_STATEFULSET_REALITY_TEST_PATTERN": "TestMinikubeStatefulSetRollbackReality",
	}
	output, err := runShellScript(t, "./drill-statefulset-reality.sh", []string{"default"}, env)
	if err != nil {
		t.Fatalf("expected enabled reality drill to pass, got %v, output: %s", err, output)
	}
	for _, token := range []string{
		"STATEFULSET_REALITY_CONTEXT=minikube",
		"STATEFULSET_REALITY_TEST_PATTERN=TestMinikubeStatefulSetRollbackReality",
		"GO_TEST_ARGS=test ./internal/healing -run TestMinikubeStatefulSetRollbackReality -count=1 -v",
		"STATEFULSET_REALITY_RESULT=pass",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected output to contain %q, got: %s", token, output)
		}
	}
}

func TestStatefulSetRealityDrillSkipsOutsideMinikube(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", `#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "cluster-info" ]]; then
	exit 0
fi
if [[ "$1" == "config" && "$2" == "current-context" ]]; then
	echo kind-dev
	exit 0
fi
exit 0
`)
	writeExecutable(t, binDir, "go", `#!/usr/bin/env bash
set -euo pipefail
exit 0
`)
	env := map[string]string{
		"PATH": binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY": "true",
	}
	output, err := runShellScript(t, "./drill-statefulset-reality.sh", []string{"default"}, env)
	if err != nil {
		t.Fatalf("expected non-minikube reality drill to skip cleanly, got %v, output: %s", err, output)
	}
	if !strings.Contains(output, "STATEFULSET_REALITY_REASON=context_kind-dev_is_not_minikube") {
		t.Fatalf("expected non-minikube skip reason, got: %s", output)
	}
}
