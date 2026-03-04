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
		"QUALITY_GATE_TRACE_FILE":            tracePath,
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
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
		"QUALITY_GATE_API_COMPATIBILITY_CLASS=backward-compatible",
		"QUALITY_GATE_API_RISK_LEVEL=low",
		"QUALITY_GATE_RELEASE_DECISION=allow",
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
	want := []string{"unit_test", "race_core", "vet", "lint", "crd_consistency", "api_contract_sync", "helm_sync"}
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
		"QUALITY_GATE_TRACE_FILE":            tracePath,
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "false",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
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
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
		"QUALITY_GATE_SLO_ACTION_LEVEL":      "degrade",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected mismatch to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=slo_governance") {
		t.Fatalf("expected slo governance mismatch category: %s", output)
	}
}

func TestQualityGateBlocksOnIncidentMappingMismatch(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
		"QUALITY_GATE_INCIDENT_LEVEL":        "critical",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected incident mapping mismatch to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=incident_mapping") {
		t.Fatalf("expected incident mapping category: %s", output)
	}
}

func TestQualityGateBlocksWhenRecoveryNotReady(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
		"QUALITY_GATE_RECOVERY_READY":        "false",
		"QUALITY_GATE_RECOVERY_CONDITION":    "pending_validation",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected recovery-not-ready to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=slo_recovery") {
		t.Fatalf("expected slo recovery category: %s", output)
	}
}

func TestQualityGateAlertSuppressionAndDedup(t *testing.T) {
	tempDir := t.TempDir()
	statePath := filepath.Join(tempDir, "alert.state")
	baseEnv := map[string]string{
		"QUALITY_GATE_CMD_TEST":                         "true",
		"QUALITY_GATE_CMD_RACE":                         "true",
		"QUALITY_GATE_CMD_VET":                          "true",
		"QUALITY_GATE_CMD_LINT":                         "true",
		"QUALITY_GATE_CMD_CRD_CHECK":                    "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":            "true",
		"QUALITY_GATE_CMD_HELM_SYNC":                    "true",
		"QUALITY_GATE_ALERT_STATE_FILE":                 statePath,
		"QUALITY_GATE_ALERT_SUPPRESSION_WINDOW_SECONDS": "600",
		"QUALITY_GATE_ALERT_KEY":                        "default/demo-app",
	}

	output1, err := runQualityGate(t, baseEnv)
	if err != nil {
		t.Fatalf("first quality gate run should pass: %v, output=%s", err, output1)
	}
	if !strings.Contains(output1, "QUALITY_GATE_ALERT_NOTIFY=true") || !strings.Contains(output1, "QUALITY_GATE_ALERT_SUPPRESSED_COUNT=0") {
		t.Fatalf("unexpected first suppression output: %s", output1)
	}

	output2, err := runQualityGate(t, baseEnv)
	if err != nil {
		t.Fatalf("second quality gate run should pass: %v, output=%s", err, output2)
	}
	if !strings.Contains(output2, "QUALITY_GATE_ALERT_NOTIFY=false") || !strings.Contains(output2, "QUALITY_GATE_ALERT_SUPPRESSED_COUNT=1") {
		t.Fatalf("expected suppression on repeated run: %s", output2)
	}
}

func TestQualityGateBlocksOnInvalidCompatibilityClass(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                "true",
		"QUALITY_GATE_CMD_RACE":                "true",
		"QUALITY_GATE_CMD_VET":                 "true",
		"QUALITY_GATE_CMD_LINT":                "true",
		"QUALITY_GATE_CMD_CRD_CHECK":           "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":   "true",
		"QUALITY_GATE_CMD_HELM_SYNC":           "true",
		"QUALITY_GATE_API_COMPATIBILITY_CLASS": "breaking-but-unknown",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected invalid compatibility class to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=api_contract") {
		t.Fatalf("expected api_contract category: %s", output)
	}
}

func TestQualityGateBlocksOnMigrationRequiredWithoutPlan(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                "true",
		"QUALITY_GATE_CMD_RACE":                "true",
		"QUALITY_GATE_CMD_VET":                 "true",
		"QUALITY_GATE_CMD_LINT":                "true",
		"QUALITY_GATE_CMD_CRD_CHECK":           "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":   "true",
		"QUALITY_GATE_CMD_HELM_SYNC":           "true",
		"QUALITY_GATE_API_COMPATIBILITY_CLASS": "migration-required",
		"QUALITY_GATE_MIGRATION_READY":         "true",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected migration-required without plan to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=migration_plan_missing") {
		t.Fatalf("expected migration_plan_missing reason: %s", output)
	}
}

func TestQualityGateBlocksOnHighRiskWithoutReleaseApproval(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                "true",
		"QUALITY_GATE_CMD_RACE":                "true",
		"QUALITY_GATE_CMD_VET":                 "true",
		"QUALITY_GATE_CMD_LINT":                "true",
		"QUALITY_GATE_CMD_CRD_CHECK":           "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":   "true",
		"QUALITY_GATE_CMD_HELM_SYNC":           "true",
		"QUALITY_GATE_API_COMPATIBILITY_CLASS": "backward-compatible",
		"QUALITY_GATE_API_RISK_LEVEL":          "high",
		"QUALITY_GATE_RELEASE_WINDOW_APPROVED": "false",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected high-risk without release approval to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_CATEGORY=runtime_production_gating") {
		t.Fatalf("expected runtime production gating category: %s", output)
	}
}
