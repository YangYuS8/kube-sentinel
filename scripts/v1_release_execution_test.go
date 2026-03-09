package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV1ReleaseExecutionRCSequencesSinglePath(t *testing.T) {
	workDir := t.TempDir()
	logFile := filepath.Join(workDir, "sequence.log")
	output, err := runShellScript(t, "./v1-release-execution.sh", nil, map[string]string{
		"V1_RELEASE_STAGE":         "rc",
		"V1_RELEASE_VERSION_TAG":   "v1.0.0-rc.1",
		"V1_RELEASE_WORK_DIR":      workDir,
		"V1_RELEASE_INSTALL_CMD":   "echo install >>\"$V1_RELEASE_TEST_LOG_FILE\"",
		"V1_RELEASE_DEV_CHECK_CMD": "echo dev-check >>\"$V1_RELEASE_TEST_LOG_FILE\"",
		"V1_RELEASE_SMOKE_CMD":     "echo smoke >>\"$V1_RELEASE_TEST_LOG_FILE\"",
		"V1_RELEASE_PIPELINE_CMD":  "printf 'DELIVERY_PIPELINE_RESULT=allow\nDELIVERY_PIPELINE_DECISION=allow\n'",
		"V1_RELEASE_TEST_LOG_FILE": logFile,
	})
	if err != nil {
		t.Fatalf("expected rc stage to pass, got %v output=%s", err, output)
	}
	content, readErr := os.ReadFile(logFile)
	if readErr != nil {
		t.Fatalf("read log failed: %v", readErr)
	}
	if string(content) != "install\ndev-check\nsmoke\n" {
		t.Fatalf("unexpected stage execution order: %q", string(content))
	}
	for _, token := range []string{
		"V1_RELEASE_STAGE=rc",
		"V1_RELEASE_VERSION_TAG=v1.0.0-rc.1",
		"V1_RELEASE_PLAN_CHANNEL=prerelease",
		"V1_RELEASE_PLAN_PUBLISH_LATEST=false",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected %q in output: %s", token, output)
		}
	}

	traceRaw, traceErr := os.ReadFile(filepath.Join(workDir, "release.trace"))
	if traceErr != nil {
		t.Fatalf("read trace failed: %v", traceErr)
	}
	for _, token := range []string{"install_minimal", "dev_local_loop_check", "runtime_closed_loop_smoke", "delivery_pipeline", "release_image_plan"} {
		if !strings.Contains(string(traceRaw), token) {
			t.Fatalf("expected trace to contain %q: %s", token, string(traceRaw))
		}
	}
}

func TestV1ReleaseExecutionRCRejectsStableVersion(t *testing.T) {
	output, err := runShellScript(t, "./v1-release-execution.sh", nil, map[string]string{
		"V1_RELEASE_STAGE":       "rc",
		"V1_RELEASE_VERSION_TAG": "v1.0.0",
	})
	if err == nil {
		t.Fatalf("expected rc stage with stable version to fail, output=%s", output)
	}
	if !strings.Contains(output, "rc stage requires a prerelease tag") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestV1ReleaseExecutionStableRequiresValidatedRC(t *testing.T) {
	output, err := runShellScript(t, "./v1-release-execution.sh", nil, map[string]string{
		"V1_RELEASE_STAGE":        "stable",
		"V1_RELEASE_VERSION_TAG":  "v1.0.0",
		"V1_RELEASE_PIPELINE_CMD": "printf 'DELIVERY_PIPELINE_RESULT=allow\nDELIVERY_PIPELINE_DECISION=allow\n'",
	})
	if err == nil {
		t.Fatalf("expected missing rc validation to fail, output=%s", output)
	}
	if !strings.Contains(output, "V1_RELEASE_RC_TAG") {
		t.Fatalf("unexpected output: %s", output)
	}
}

func TestV1ReleaseExecutionStablePublishesLatestAfterProofs(t *testing.T) {
	workDir := t.TempDir()
	output, err := runShellScript(t, "./v1-release-execution.sh", nil, map[string]string{
		"V1_RELEASE_STAGE":            "stable",
		"V1_RELEASE_VERSION_TAG":      "v1.0.0",
		"V1_RELEASE_RC_TAG":           "v1.0.0-rc.1",
		"V1_RELEASE_PREPROD_VERIFIED": "true",
		"V1_RELEASE_PILOT_VERIFIED":   "true",
		"V1_RELEASE_GO_LIVE_VERIFIED": "true",
		"V1_RELEASE_WORK_DIR":         workDir,
		"V1_RELEASE_PIPELINE_CMD":     "printf 'DELIVERY_PIPELINE_RESULT=allow\nDELIVERY_PIPELINE_DECISION=allow\n'",
	})
	if err != nil {
		t.Fatalf("expected stable stage to pass, got %v output=%s", err, output)
	}
	for _, token := range []string{
		"V1_RELEASE_STAGE=stable",
		"V1_RELEASE_PLAN_CHANNEL=stable",
		"V1_RELEASE_PLAN_PUBLISH_LATEST=true",
		"V1_RELEASE_PREPROD_VERIFIED=true",
		"V1_RELEASE_PILOT_VERIFIED=true",
		"V1_RELEASE_GO_LIVE_VERIFIED=true",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected %q in output: %s", token, output)
		}
	}
}
