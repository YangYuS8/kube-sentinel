package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runDeliveryPipeline(t *testing.T, env map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "./delivery-pipeline.sh")
	cmd.Dir = "."
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestDeliveryPipelineSuccessAllow(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=all_checks_passed\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=all_green\nDRY_RUN_TRACE_KEY=trace-allow\n'",
	})
	if err != nil {
		t.Fatalf("pipeline should succeed: %v output=%s", err, output)
	}
	for _, token := range []string{"DELIVERY_PIPELINE_RESULT=allow", "DELIVERY_PIPELINE_FAILED_STAGE=none", "DELIVERY_PIPELINE_REASON=all_stages_completed"} {
		if !strings.Contains(output, token) {
			t.Fatalf("missing %s in output: %s", token, output)
		}
	}
}

func TestDeliveryPipelineStopsOnQualityFailure(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "false",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ignored\nDRY_RUN_TRACE_KEY=trace-ignored\n'",
	})
	if err == nil {
		t.Fatalf("pipeline should fail when quality gate fails: %s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILED_STAGE=quality_gate") {
		t.Fatalf("expected quality_gate failed stage, output=%s", output)
	}
	traceRaw, readErr := os.ReadFile(filepath.Join(workDir, "delivery-pipeline.trace"))
	if readErr != nil {
		t.Fatalf("read trace failed: %v", readErr)
	}
	if strings.Contains(string(traceRaw), "preprod_dry_run") {
		t.Fatalf("dry-run stage should not execute after quality failure: %s", string(traceRaw))
	}
}

func TestDeliveryPipelineStopsOnDryRunFailure(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "false",
	})
	if err == nil {
		t.Fatalf("pipeline should fail when dry-run fails: %s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILED_STAGE=preprod_dry_run") {
		t.Fatalf("expected dry-run failed stage, output=%s", output)
	}
}

func TestDeliveryPipelineBlocksWhenEvidenceMissingFields(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-ok\n'",
	})
	if err == nil {
		t.Fatalf("expected evidence missing fields to fail: %s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_REASON=quality_gate_evidence_missing_fields") {
		t.Fatalf("unexpected reason: %s", output)
	}
}

func TestDeliveryPipelineDryRunOutcomeCoverage(t *testing.T) {
	cases := []struct {
		name          string
		dryRunOutcome string
		expectResult  string
		expectFail    bool
	}{
		{name: "allow", dryRunOutcome: "allow", expectResult: "allow", expectFail: false},
		{name: "degrade", dryRunOutcome: "degrade", expectResult: "degrade", expectFail: false},
		{name: "block", dryRunOutcome: "block", expectResult: "block", expectFail: false},
		{name: "invalid", dryRunOutcome: "UNKNOWN", expectResult: "block", expectFail: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			workDir := t.TempDir()
			output, err := runDeliveryPipeline(t, map[string]string{
				"DELIVERY_PIPELINE_WORK_DIR":         workDir,
				"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\n'",
				"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=" + tc.dryRunOutcome + "\nDRY_RUN_REASON=simulated\nDRY_RUN_TRACE_KEY=trace-" + tc.name + "\n'",
			})
			if tc.expectFail && err == nil {
				t.Fatalf("expected failure for %s, output=%s", tc.name, output)
			}
			if !tc.expectFail && err != nil {
				t.Fatalf("expected success for %s: err=%v output=%s", tc.name, err, output)
			}
			if !strings.Contains(output, "DELIVERY_PIPELINE_RESULT="+tc.expectResult) {
				t.Fatalf("expected result %s, output=%s", tc.expectResult, output)
			}
		})
	}
}

func TestDeliveryPipelineOncallConsistencyValidation(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=block\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=simulated\nDRY_RUN_TRACE_KEY=trace-conflict\n'",
		"ONCALL_ALERT_LEVEL":                 "warning",
		"ONCALL_RUNBOOK":                     "runbook://runtime-degrade-recovery",
		"ONCALL_APPROVAL_TRIGGER":            "oncall_ack_required",
	})
	if err == nil {
		t.Fatalf("expected oncall consistency failure, output=%s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILED_STAGE=oncall_consistency") {
		t.Fatalf("unexpected output=%s", output)
	}
}

func TestDeliveryPipelineOperatorOverrideAudit(t *testing.T) {
	workDir := t.TempDir()
	overrideFile := filepath.Join(workDir, "override-idempotency.log")
	auditFile := filepath.Join(workDir, "override-audit.log")

	baseEnv := map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                  workDir,
		"DELIVERY_PIPELINE_OVERRIDE_IDEMPOTENCY_FILE": overrideFile,
		"DELIVERY_PIPELINE_OVERRIDE_AUDIT_FILE":       auditFile,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":          "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":               "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=simulated\nDRY_RUN_TRACE_KEY=trace-override\n'",
		"DELIVERY_PIPELINE_OPERATOR_OVERRIDE":         "true",
		"DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP":        "2026-03-04T05:00:00Z",
		"DELIVERY_PIPELINE_OVERRIDE_ACTOR":            "oncall-a",
		"DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION":     "degrade",
		"DELIVERY_PIPELINE_OVERRIDE_POST_DECISION":    "allow",
		"DELIVERY_PIPELINE_OVERRIDE_REASON":           "validated by operator",
		"DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY":        "override-key-1",
	}

	output1, err := runDeliveryPipeline(t, baseEnv)
	if err != nil {
		t.Fatalf("expected override with complete fields to pass: %v output=%s", err, output1)
	}
	if !strings.Contains(output1, "DELIVERY_PIPELINE_RESULT=allow") {
		t.Fatalf("unexpected first result: %s", output1)
	}
	if !strings.Contains(output1, "DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=recorded") {
		t.Fatalf("expected recorded state: %s", output1)
	}

	output2, err := runDeliveryPipeline(t, baseEnv)
	if err != nil {
		t.Fatalf("expected idempotent replay to pass: %v output=%s", err, output2)
	}
	if !strings.Contains(output2, "DELIVERY_PIPELINE_OPERATOR_OVERRIDE_AUDIT_STATE=idempotent") {
		t.Fatalf("expected idempotent state: %s", output2)
	}

	output3, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":          workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":  baseEnv["DELIVERY_PIPELINE_QUALITY_GATE_CMD"],
		"DELIVERY_PIPELINE_DRY_RUN_CMD":       baseEnv["DELIVERY_PIPELINE_DRY_RUN_CMD"],
		"DELIVERY_PIPELINE_OPERATOR_OVERRIDE": "true",
	})
	if err == nil {
		t.Fatalf("expected missing override fields to fail: %s", output3)
	}
	if !strings.Contains(output3, "DELIVERY_PIPELINE_FAILED_STAGE=operator_override_audit") {
		t.Fatalf("expected override audit failure stage: %s", output3)
	}
}
