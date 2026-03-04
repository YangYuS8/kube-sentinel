package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runQualityGate(t *testing.T, env map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "./quality-gate.sh")
	cmd.Dir = "."
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestQualityGateOrderAllPass(t *testing.T) {
	tempDir := t.TempDir()
	tracePath := filepath.Join(tempDir, "trace.log")
	env := map[string]string{
		"QUALITY_GATE_TRACE_FILE":    tracePath,
		"QUALITY_GATE_CMD_TEST":      "true",
		"QUALITY_GATE_CMD_RACE":      "true",
		"QUALITY_GATE_CMD_VET":       "true",
		"QUALITY_GATE_CMD_LINT":      "true",
		"QUALITY_GATE_CMD_CRD_CHECK": "true",
		"QUALITY_GATE_CMD_HELM_SYNC": "true",
	}
	output, err := runQualityGate(t, env)
	if err != nil {
		t.Fatalf("quality gate should pass, got error: %v, output: %s", err, output)
	}
	if !strings.Contains(output, "QUALITY_GATE_RESULT=allow") {
		t.Fatalf("expected allow result, output: %s", output)
	}
	for _, token := range []string{
		"QUALITY_GATE_SLO_ACTION_LEVEL=allow",
		"QUALITY_GATE_SLO_BUDGET_STATUS=healthy",
		"QUALITY_GATE_INCIDENT_LEVEL=info",
		"QUALITY_GATE_RECOVERY_CONDITION=maintain_target_and_observe",
		"QUALITY_GATE_RUNBOOK=runbook://runtime-observation",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("expected token %s in output: %s", token, output)
		}
	}
	traceRaw, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace failed: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(traceRaw)), "\n")
	want := []string{"unit_test", "race_core", "vet", "lint", "crd_consistency", "helm_sync"}
	if len(got) != len(want) {
		t.Fatalf("unexpected step count: got %d want %d (%v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected step order at %d: got %s want %s", index, got[index], want[index])
		}
	}
}

func TestQualityGateStopsOnFirstFailure(t *testing.T) {
	tempDir := t.TempDir()
	tracePath := filepath.Join(tempDir, "trace.log")
	env := map[string]string{
		"QUALITY_GATE_TRACE_FILE":    tracePath,
		"QUALITY_GATE_CMD_TEST":      "true",
		"QUALITY_GATE_CMD_RACE":      "true",
		"QUALITY_GATE_CMD_VET":       "true",
		"QUALITY_GATE_CMD_LINT":      "false",
		"QUALITY_GATE_CMD_CRD_CHECK": "true",
		"QUALITY_GATE_CMD_HELM_SYNC": "true",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("quality gate should fail when lint fails, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=lint") {
		t.Fatalf("expected lint category in failure output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_SLO_ACTION_LEVEL=block") {
		t.Fatalf("expected block slo action level in failure output: %s", output)
	}
	traceRaw, err := os.ReadFile(tracePath)
	if err != nil {
		t.Fatalf("read trace failed: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(traceRaw)), "\n")
	want := []string{"unit_test", "race_core", "vet", "lint"}
	if len(got) != len(want) {
		t.Fatalf("unexpected step count after short-circuit: got %d want %d (%v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected step order at %d: got %s want %s", index, got[index], want[index])
		}
	}
}

func TestQualityGateBlocksOnSLOSemanticMismatch(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":         "true",
		"QUALITY_GATE_CMD_RACE":         "true",
		"QUALITY_GATE_CMD_VET":          "true",
		"QUALITY_GATE_CMD_LINT":         "true",
		"QUALITY_GATE_CMD_CRD_CHECK":    "true",
		"QUALITY_GATE_CMD_HELM_SYNC":    "true",
		"QUALITY_GATE_SLO_ACTION_LEVEL": "degrade",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected mismatch to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=slo_governance") {
		t.Fatalf("expected slo governance mismatch category: %s", output)
	}
}
