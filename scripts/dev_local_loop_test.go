package scripts

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fakeKubectlScript() string {
	return `#!/usr/bin/env bash
set -euo pipefail

if [[ "${KUBE_SENTINEL_TEST_CONTEXT_EMPTY:-false}" == "true" ]] && [[ "$1" == "config" ]] && [[ "$2" == "current-context" ]]; then
  exit 0
fi

if [[ "$1" == "config" && "$2" == "current-context" ]]; then
	echo minikube
	exit 0
fi

if [[ "$1" == "cluster-info" ]]; then
	exit 0
fi

if [[ "$1" == "-n" && "$2" == "kube-system" && "$3" == "get" && "$4" == "pods" ]]; then
	if [[ "${KUBE_SENTINEL_TEST_KUBE_SYSTEM_PENDING:-false}" == "true" ]]; then
		cat <<'EOF'
coredns-1 1/1 Pending 0 10s
storage-provisioner 1/1 Running 0 10s
EOF
	else
		cat <<'EOF'
coredns-1 1/1 Running 0 10s
storage-provisioner 1/1 Running 0 10s
EOF
	fi
	exit 0
fi

if [[ "$1" == "get" && "$2" == "crd" && "$3" == "healingrequests.kubesentinel.io" ]]; then
	if [[ "${KUBE_SENTINEL_TEST_CRD_MISSING:-false}" == "true" ]]; then
		exit 1
	fi
	exit 0
fi

if [[ "$1" == "-n" && "$2" == "default" && "$3" == "get" && "$4" == "deployment" && "$5" == "demo-app" ]]; then
	if [[ "${KUBE_SENTINEL_TEST_DEMO_MISSING:-false}" == "true" ]]; then
		exit 1
	fi
	exit 0
fi

if [[ "$1" == "-n" && "$2" == "kube-sentinel-system" && "$3" == "get" && "$4" == "svc" && "$5" == "kube-sentinel" ]]; then
	exit 0
fi

if [[ "$1" == "apply" && "$2" == "-f" && "$3" == "-" ]]; then
	echo demo-applied >>"${KUBE_SENTINEL_TEST_LOG_FILE}"
	cat >/dev/null
	exit 0
fi

if [[ "$1" == "apply" && "$2" == "-f" && "$3" == *"_healingrequests.yaml" ]]; then
	echo crd-applied >>"${KUBE_SENTINEL_TEST_LOG_FILE}"
	exit 0
fi

exit 0
`
}

func TestDevLocalLoopFailsWhenContextMissing(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", fakeKubectlScript())
	writeExecutable(t, binDir, "curl", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeExecutable(t, binDir, "go", "#!/usr/bin/env bash\nset -euo pipefail\n")

	output, err := runShellScript(t, "./dev-local-loop.sh", []string{"check"}, map[string]string{
		"PATH":                             binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_TEST_CONTEXT_EMPTY": "true",
	})
	if err == nil {
		t.Fatalf("expected missing context to fail, output: %s", output)
	}
	if !strings.Contains(output, "kubectl current-context 为空") {
		t.Fatalf("expected context failure message, output: %s", output)
	}
}

func TestDevLocalLoopInstallsCRDAndDemoOnCheck(t *testing.T) {
	binDir := t.TempDir()
	logFile := filepath.Join(t.TempDir(), "actions.log")
	writeExecutable(t, binDir, "kubectl", fakeKubectlScript())
	writeExecutable(t, binDir, "curl", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeExecutable(t, binDir, "go", "#!/usr/bin/env bash\nset -euo pipefail\n")

	output, err := runShellScript(t, "./dev-local-loop.sh", []string{"check"}, map[string]string{
		"PATH":                            binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_TEST_CRD_MISSING":  "true",
		"KUBE_SENTINEL_TEST_DEMO_MISSING": "true",
		"KUBE_SENTINEL_TEST_LOG_FILE":     logFile,
	})
	if err != nil {
		t.Fatalf("expected check mode to pass, got %v, output: %s", err, output)
	}
	if !strings.Contains(output, "next steps") {
		t.Fatalf("expected next steps output, got: %s", output)
	}
	content, readErr := os.ReadFile(logFile)
	if readErr != nil {
		t.Fatalf("read action log failed: %v", readErr)
	}
	for _, token := range []string{"crd-applied", "demo-applied"} {
		if !strings.Contains(string(content), token) {
			t.Fatalf("expected log to contain %q, got: %s", token, string(content))
		}
	}
}

func TestDevLocalLoopFailsWhenPortOccupied(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:8090")
	if err != nil {
		t.Fatalf("listen on 8090 failed: %v", err)
	}
	defer func() {
		if closeErr := listener.Close(); closeErr != nil {
			t.Fatalf("close listener failed: %v", closeErr)
		}
	}()

	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", fakeKubectlScript())
	writeExecutable(t, binDir, "curl", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeExecutable(t, binDir, "go", "#!/usr/bin/env bash\nset -euo pipefail\n")

	output, runErr := runShellScript(t, "./dev-local-loop.sh", []string{"run-local"}, map[string]string{
		"PATH":                           binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_DEV_LOOP_DRY_RUN": "true",
	})
	if runErr == nil {
		t.Fatalf("expected occupied port to fail, output: %s", output)
	}
	if !strings.Contains(output, "port 8080 已被占用") && !strings.Contains(output, "port 8090 已被占用") {
		t.Fatalf("expected port failure output, got: %s", output)
	}
}

func TestDevLocalLoopFailsWhenBaseComponentsNotReady(t *testing.T) {
	binDir := t.TempDir()
	writeExecutable(t, binDir, "kubectl", fakeKubectlScript())
	writeExecutable(t, binDir, "curl", "#!/usr/bin/env bash\nset -euo pipefail\n")
	writeExecutable(t, binDir, "go", "#!/usr/bin/env bash\nset -euo pipefail\n")

	output, err := runShellScript(t, "./dev-local-loop.sh", []string{"check"}, map[string]string{
		"PATH":                                   binDir + ":" + os.Getenv("PATH"),
		"KUBE_SENTINEL_TEST_KUBE_SYSTEM_PENDING": "true",
	})
	if err == nil {
		t.Fatalf("expected pending kube-system components to fail, output: %s", output)
	}
	if !strings.Contains(output, "coredns 尚未 Running") {
		t.Fatalf("expected kube-system readiness failure, output: %s", output)
	}
}
