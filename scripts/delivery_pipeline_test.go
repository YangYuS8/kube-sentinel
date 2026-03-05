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
		{name: "degrade", dryRunOutcome: "degrade", expectResult: "block", expectFail: false},
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

func TestDeliveryPipelineGoLiveGatePriority(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                                 workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":                         "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r1\nQUALITY_GATE_DRILL_SUCCESS_RATE=0.70\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=400000\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":                              "printf 'DRY_RUN_OUTCOME=block\nDRY_RUN_REASON=stability-regression\nDRY_RUN_TRACE_KEY=trace-priority\n'",
		"DELIVERY_PIPELINE_DRILL_MIN_SUCCESS_RATE":                   "0.95",
		"DELIVERY_PIPELINE_DRILL_MAX_ROLLBACK_P95_MS":                "300000",
		"DELIVERY_PIPELINE_PREPROD_EVIDENCE_TTL_SECONDS":             "3600",
		"DELIVERY_PIPELINE_PREPROD_EVIDENCE_TIMESTAMP_EPOCH_SECONDS": "1700000000",
		"DELIVERY_PIPELINE_NOW_EPOCH_SECONDS":                        "1700000300",
	})
	if err != nil {
		t.Fatalf("expected pipeline script to complete with block decision, err=%v output=%s", err, output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_RESULT=block") {
		t.Fatalf("expected block result, output=%s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILURE_CATEGORY=stability") {
		t.Fatalf("expected stability to win priority, output=%s", output)
	}
}

func TestDeliveryPipelineBlocksWhenPreprodEvidenceExpired(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                                 workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":                         "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r2\nQUALITY_GATE_DRILL_SUCCESS_RATE=1.0\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=100\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":                              "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-expired\n'",
		"DELIVERY_PIPELINE_PREPROD_EVIDENCE_TIMESTAMP_EPOCH_SECONDS": "1700000000",
		"DELIVERY_PIPELINE_PREPROD_EVIDENCE_TTL_SECONDS":             "60",
		"DELIVERY_PIPELINE_NOW_EPOCH_SECONDS":                        "1700000800",
	})
	if err != nil {
		t.Fatalf("expected completed run with block decision: %v output=%s", err, output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_GATE_STABILITY_STATUS=fail") {
		t.Fatalf("expected stability gate fail, output=%s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILURE_CATEGORY=stability") {
		t.Fatalf("expected stability failure category, output=%s", output)
	}
}

func TestDeliveryPipelineRollbackDrillThresholds(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                  workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":          "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r3\nQUALITY_GATE_DRILL_SUCCESS_RATE=0.94\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=301000\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":               "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-drill\n'",
		"DELIVERY_PIPELINE_DRILL_MIN_SUCCESS_RATE":    "0.95",
		"DELIVERY_PIPELINE_DRILL_MAX_ROLLBACK_P95_MS": "300000",
	})
	if err != nil {
		t.Fatalf("expected completed run with block decision: %v output=%s", err, output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_GATE_DRILL_ROLLBACK_STATUS=fail") {
		t.Fatalf("expected drill gate fail, output=%s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILURE_CATEGORY=drill_rollback") {
		t.Fatalf("expected drill failure category, output=%s", output)
	}
}

func TestDeliveryPipelineApprovalMismatchBlocks(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=block\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r4\nQUALITY_GATE_DRILL_SUCCESS_RATE=1.0\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=100\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-approval\n'",
		"ONCALL_APPROVAL_LEVEL":              "observe_only",
	})
	if err != nil {
		t.Fatalf("expected completed run with block decision: %v output=%s", err, output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=fail") {
		t.Fatalf("expected approval gate fail, output=%s", output)
	}
}

func TestDeliveryPipelineFreezeWindowBoundaryOverrideBlocked(t *testing.T) {
	workDir := t.TempDir()
	baseEnv := map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                          workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":                  "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r5\nQUALITY_GATE_DRILL_SUCCESS_RATE=1.0\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=100\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":                       "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-freeze\n'",
		"DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS": "1700000100",
		"DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS":   "1700000200",
		"DELIVERY_PIPELINE_OPERATOR_OVERRIDE":                 "true",
		"DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP":                "2026-03-05T00:00:00Z",
		"DELIVERY_PIPELINE_OVERRIDE_ACTOR":                    "oncall-b",
		"DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION":             "block",
		"DELIVERY_PIPELINE_OVERRIDE_POST_DECISION":            "allow",
		"DELIVERY_PIPELINE_OVERRIDE_REASON":                   "manual unblock",
		"DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY":                "trace-freeze-override",
	}

	insideOutput, insideErr := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                          baseEnv["DELIVERY_PIPELINE_WORK_DIR"],
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":                  baseEnv["DELIVERY_PIPELINE_QUALITY_GATE_CMD"],
		"DELIVERY_PIPELINE_DRY_RUN_CMD":                       baseEnv["DELIVERY_PIPELINE_DRY_RUN_CMD"],
		"DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS": baseEnv["DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS"],
		"DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS":   baseEnv["DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS"],
		"DELIVERY_PIPELINE_OPERATOR_OVERRIDE":                 baseEnv["DELIVERY_PIPELINE_OPERATOR_OVERRIDE"],
		"DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP":                baseEnv["DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP"],
		"DELIVERY_PIPELINE_OVERRIDE_ACTOR":                    baseEnv["DELIVERY_PIPELINE_OVERRIDE_ACTOR"],
		"DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION":             baseEnv["DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION"],
		"DELIVERY_PIPELINE_OVERRIDE_POST_DECISION":            baseEnv["DELIVERY_PIPELINE_OVERRIDE_POST_DECISION"],
		"DELIVERY_PIPELINE_OVERRIDE_REASON":                   baseEnv["DELIVERY_PIPELINE_OVERRIDE_REASON"],
		"DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY":                baseEnv["DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY"],
		"DELIVERY_PIPELINE_NOW_EPOCH_SECONDS":                 "1700000100",
	})
	if insideErr != nil {
		t.Fatalf("expected completed run with block decision: %v output=%s", insideErr, insideOutput)
	}
	if !strings.Contains(insideOutput, "DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=fail") {
		t.Fatalf("expected freeze boundary block, output=%s", insideOutput)
	}

	outsideOutput, outsideErr := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":                          baseEnv["DELIVERY_PIPELINE_WORK_DIR"],
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":                  baseEnv["DELIVERY_PIPELINE_QUALITY_GATE_CMD"],
		"DELIVERY_PIPELINE_DRY_RUN_CMD":                       baseEnv["DELIVERY_PIPELINE_DRY_RUN_CMD"],
		"DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS": baseEnv["DELIVERY_PIPELINE_FREEZE_WINDOW_START_EPOCH_SECONDS"],
		"DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS":   baseEnv["DELIVERY_PIPELINE_FREEZE_WINDOW_END_EPOCH_SECONDS"],
		"DELIVERY_PIPELINE_OPERATOR_OVERRIDE":                 baseEnv["DELIVERY_PIPELINE_OPERATOR_OVERRIDE"],
		"DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP":                baseEnv["DELIVERY_PIPELINE_OVERRIDE_TIMESTAMP"],
		"DELIVERY_PIPELINE_OVERRIDE_ACTOR":                    baseEnv["DELIVERY_PIPELINE_OVERRIDE_ACTOR"],
		"DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION":             baseEnv["DELIVERY_PIPELINE_OVERRIDE_PRE_DECISION"],
		"DELIVERY_PIPELINE_OVERRIDE_POST_DECISION":            baseEnv["DELIVERY_PIPELINE_OVERRIDE_POST_DECISION"],
		"DELIVERY_PIPELINE_OVERRIDE_REASON":                   baseEnv["DELIVERY_PIPELINE_OVERRIDE_REASON"],
		"DELIVERY_PIPELINE_OVERRIDE_TRACE_KEY":                "trace-freeze-override-outside",
		"DELIVERY_PIPELINE_NOW_EPOCH_SECONDS":                 "1700000201",
	})
	if outsideErr != nil {
		t.Fatalf("expected allow outside freeze window: %v output=%s", outsideErr, outsideOutput)
	}
	if !strings.Contains(outsideOutput, "DELIVERY_PIPELINE_GATE_APPROVAL_FREEZE_STATUS=pass") {
		t.Fatalf("expected freeze gate pass outside window, output=%s", outsideOutput)
	}
}

func TestDeliveryPipelineDecisionPackMissingFieldsBlocks(t *testing.T) {
	workDir := t.TempDir()
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":         workDir,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD": "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r6\nQUALITY_GATE_DRILL_SUCCESS_RATE=1.0\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=100\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":      "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=\n'",
	})
	if err == nil {
		t.Fatalf("expected failure when required evidence fields are missing: %s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_FAILED_STAGE=preprod_dry_run") {
		t.Fatalf("expected preprod evidence failure stage, output=%s", output)
	}
	if !strings.Contains(output, "DELIVERY_PIPELINE_REASON=dry_run_evidence_missing_fields") {
		t.Fatalf("expected dry_run_evidence_missing_fields reason, output=%s", output)
	}
}

func TestDeliveryPipelineDecisionPackContractStable(t *testing.T) {
	workDir := t.TempDir()
	decisionPack := filepath.Join(workDir, "release-decision-pack.json")
	output, err := runDeliveryPipeline(t, map[string]string{
		"DELIVERY_PIPELINE_WORK_DIR":           workDir,
		"DELIVERY_PIPELINE_DECISION_PACK_FILE": decisionPack,
		"DELIVERY_PIPELINE_QUALITY_GATE_CMD":   "printf 'QUALITY_GATE_RESULT=allow\nQUALITY_GATE_CATEGORY=quality_gate\nQUALITY_GATE_REASON=ok\nQUALITY_GATE_FIX_HINT=n/a\nQUALITY_GATE_RELEASE_DECISION=allow\nQUALITY_GATE_RELEASE_READINESS_ROLLBACK_CANDIDATE=stable-r7\nQUALITY_GATE_DRILL_SUCCESS_RATE=1.0\nQUALITY_GATE_DRILL_ROLLBACK_P95_MS=100\n'",
		"DELIVERY_PIPELINE_DRY_RUN_CMD":        "printf 'DRY_RUN_OUTCOME=allow\nDRY_RUN_REASON=ok\nDRY_RUN_TRACE_KEY=trace-pack\n'",
	})
	if err != nil {
		t.Fatalf("expected allow run: %v output=%s", err, output)
	}
	raw, readErr := os.ReadFile(decisionPack)
	if readErr != nil {
		t.Fatalf("read decision pack failed: %v", readErr)
	}
	for _, token := range []string{"\"decision\"", "\"failureCategory\"", "\"rollbackCandidate\"", "\"drillSummary\"", "\"approval\"", "\"correlationKey\"", "\"timestamp\""} {
		if !strings.Contains(string(raw), token) {
			t.Fatalf("decision pack missing %s: %s", token, string(raw))
		}
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
