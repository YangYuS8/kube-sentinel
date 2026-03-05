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
	cmd.Env = append(os.Environ(), "QUALITY_GATE_CMD_CHANGE_SPLIT_GOVERNANCE=true")
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
		"QUALITY_GATE_RELEASE_READINESS_ACTION_TYPE=restart",
		"QUALITY_GATE_RELEASE_READINESS_DECISION=allow",
		"QUALITY_GATE_GO_LIVE_DECISION=allow",
		"QUALITY_GATE_GO_LIVE_FAILURE_CATEGORY=none",
		"QUALITY_GATE_GO_LIVE_ROLLOUT_RECOMMENDATION=proceed",
		"QUALITY_GATE_DRILL_SUCCESS_RATE=1.0",
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
	want := []string{"unit_test", "race_core", "vet", "lint", "crd_consistency", "api_contract_sync", "helm_sync", "change_splitting_governance"}
	if len(got) != len(want) {
		t.Fatalf("unexpected step count: got %d want %d (%v)", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("unexpected step order at %d: got %s want %s", index, got[index], want[index])
		}
	}
}

func TestQualityGateBlocksWhenReleaseReadinessCandidateMissing(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                             "true",
		"QUALITY_GATE_CMD_RACE":                             "true",
		"QUALITY_GATE_CMD_VET":                              "true",
		"QUALITY_GATE_CMD_LINT":                             "true",
		"QUALITY_GATE_CMD_CRD_CHECK":                        "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":                "true",
		"QUALITY_GATE_CMD_HELM_SYNC":                        "true",
		"QUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE": "",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected missing rollback candidate to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=release_readiness_missing_rollback_candidate") {
		t.Fatalf("expected missing rollback reason, output: %s", output)
	}
}

func TestQualityGateBlocksWhenReleaseReadinessIncidentsExceeded(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                         "true",
		"QUALITY_GATE_CMD_RACE":                         "true",
		"QUALITY_GATE_CMD_VET":                          "true",
		"QUALITY_GATE_CMD_LINT":                         "true",
		"QUALITY_GATE_CMD_CRD_CHECK":                    "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":            "true",
		"QUALITY_GATE_CMD_HELM_SYNC":                    "true",
		"QUALITY_GATE_RELEASE_READINESS_OPEN_INCIDENTS": "4",
		"QUALITY_GATE_RELEASE_MAX_OPEN_INCIDENTS":       "3",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected incidents exceeded to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=release_readiness_open_incidents_exceeded") {
		t.Fatalf("expected incident exceeded reason, output: %s", output)
	}
}

func TestQualityGateBlocksWhenDecisionNotAllowedByPolicy(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
		"QUALITY_GATE_ALLOWED_DECISIONS":     "degrade,block",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected policy mismatch to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=release_decision_not_allowed_by_policy") {
		t.Fatalf("expected policy mismatch reason, output: %s", output)
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

func TestQualityGateWritesEvidenceArtifactsWhenConfigured(t *testing.T) {
	tempDir := t.TempDir()
	evidencePath := filepath.Join(tempDir, "quality-evidence.json")
	summaryPath := filepath.Join(tempDir, "quality-summary.txt")
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":              "true",
		"QUALITY_GATE_CMD_RACE":              "true",
		"QUALITY_GATE_CMD_VET":               "true",
		"QUALITY_GATE_CMD_LINT":              "true",
		"QUALITY_GATE_CMD_CRD_CHECK":         "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC": "true",
		"QUALITY_GATE_CMD_HELM_SYNC":         "true",
		"QUALITY_GATE_EVIDENCE_JSON_FILE":    evidencePath,
		"QUALITY_GATE_SUMMARY_FILE":          summaryPath,
	}

	output, err := runQualityGate(t, env)
	if err != nil {
		t.Fatalf("quality gate should pass: %v output=%s", err, output)
	}
	evidenceRaw, readErr := os.ReadFile(evidencePath)
	if readErr != nil {
		t.Fatalf("read evidence failed: %v", readErr)
	}
	for _, token := range []string{"\"result\": \"allow\"", "\"category\": \"quality_gate\"", "\"reasonCode\": \"all_checks_passed\"", "\"fixHint\": \"n/a\""} {
		if !strings.Contains(string(evidenceRaw), token) {
			t.Fatalf("evidence missing %s: %s", token, string(evidenceRaw))
		}
	}
	summaryRaw, summaryErr := os.ReadFile(summaryPath)
	if summaryErr != nil {
		t.Fatalf("read summary failed: %v", summaryErr)
	}
	for _, token := range []string{"QUALITY_GATE_RESULT=allow", "QUALITY_GATE_CATEGORY=quality_gate", "QUALITY_GATE_REASON=all_checks_passed", "QUALITY_GATE_FIX_HINT=n/a"} {
		if !strings.Contains(string(summaryRaw), token) {
			t.Fatalf("summary missing %s: %s", token, string(summaryRaw))
		}
	}
}

func TestQualityGateEvidenceMissingRequiredFieldsBlocksPipeline(t *testing.T) {
	workDir := t.TempDir()
	cmd := exec.Command("bash", "./delivery-pipeline.sh")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(),
		"DELIVERY_PIPELINE_WORK_DIR="+workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD=printf 'QUALITY_GATE_RESULT=allow\\nQUALITY_GATE_CATEGORY=quality_gate\\nQUALITY_GATE_REASON=ok\\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD=printf 'DRY_RUN_OUTCOME=allow\\nDRY_RUN_REASON=ok\\nDRY_RUN_TRACE_KEY=trace-ok\\n'",
	)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected pipeline to block when evidence fields are missing, output=%s", string(output))
	}
	if !strings.Contains(string(output), "DELIVERY_PIPELINE_REASON=quality_gate_evidence_missing_fields") {
		t.Fatalf("unexpected pipeline output: %s", string(output))
	}
}

func TestQualityGateBlocksOnChangeSplitGovernanceViolation(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                    "true",
		"QUALITY_GATE_CMD_RACE":                    "true",
		"QUALITY_GATE_CMD_VET":                     "true",
		"QUALITY_GATE_CMD_LINT":                    "true",
		"QUALITY_GATE_CMD_CRD_CHECK":               "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":       "true",
		"QUALITY_GATE_CMD_HELM_SYNC":               "true",
		"QUALITY_GATE_CMD_CHANGE_SPLIT_GOVERNANCE": "bash ./check-change-splitting-governance.sh",
		"CHANGE_SPLIT_GOVERNANCE_ENABLED":          "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":            "3",
		"CHANGE_SPLIT_INCREMENT_ITEMS":             "11",
		"CHANGE_SPLIT_RISK_DOMAINS":                "blocking",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected governance violation to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=change_splitting_governance_violation") {
		t.Fatalf("expected governance violation reason, output: %s", output)
	}
}

func TestQualityGateAllowsWhenChangeSplitPlanProvided(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                    "true",
		"QUALITY_GATE_CMD_RACE":                    "true",
		"QUALITY_GATE_CMD_VET":                     "true",
		"QUALITY_GATE_CMD_LINT":                    "true",
		"QUALITY_GATE_CMD_CRD_CHECK":               "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":       "true",
		"QUALITY_GATE_CMD_HELM_SYNC":               "true",
		"QUALITY_GATE_CMD_CHANGE_SPLIT_GOVERNANCE": "bash ./check-change-splitting-governance.sh",
		"CHANGE_SPLIT_GOVERNANCE_ENABLED":          "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":            "3",
		"CHANGE_SPLIT_INCREMENT_ITEMS":             "11",
		"CHANGE_SPLIT_RISK_DOMAINS":                "blocking",
		"CHANGE_SPLIT_HAS_SPLIT_PLAN":              "true",
		"CHANGE_SPLIT_PLAN_REF":                    "change-a,change-b",
	}
	output, err := runQualityGate(t, env)
	if err != nil {
		t.Fatalf("expected split plan to pass: %v output=%s", err, output)
	}
	if !strings.Contains(output, "QUALITY_GATE_RESULT=allow") {
		t.Fatalf("expected allow result, output: %s", output)
	}
}

func TestQualityGateBlocksOnChangeSplitExceptionFieldsMissing(t *testing.T) {
	env := map[string]string{
		"QUALITY_GATE_CMD_TEST":                    "true",
		"QUALITY_GATE_CMD_RACE":                    "true",
		"QUALITY_GATE_CMD_VET":                     "true",
		"QUALITY_GATE_CMD_LINT":                    "true",
		"QUALITY_GATE_CMD_CRD_CHECK":               "true",
		"QUALITY_GATE_CMD_API_CONTRACT_SYNC":       "true",
		"QUALITY_GATE_CMD_HELM_SYNC":               "true",
		"QUALITY_GATE_CMD_CHANGE_SPLIT_GOVERNANCE": "bash ./check-change-splitting-governance.sh",
		"CHANGE_SPLIT_GOVERNANCE_ENABLED":          "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":            "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":             "6",
		"CHANGE_SPLIT_RISK_DOMAINS":                "blocking,operational",
		"CHANGE_SPLIT_EXCEPTION_APPROVED":          "true",
	}
	output, err := runQualityGate(t, env)
	if err == nil {
		t.Fatalf("expected missing exception fields to fail, output: %s", output)
	}
	if !strings.Contains(output, "QUALITY_GATE_REASON=change_splitting_governance_violation") {
		t.Fatalf("expected governance violation reason, output: %s", output)
	}
}
