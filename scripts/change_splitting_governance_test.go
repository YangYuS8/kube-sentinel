package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runChangeSplitGovernance(t *testing.T, env map[string]string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "./check-change-splitting-governance.sh")
	cmd.Dir = "."
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestChangeSplitGovernanceBoundaries(t *testing.T) {
	base := map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": "true",
		"CHANGE_SPLIT_RISK_DOMAINS":       "blocking",
	}

	allow2, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": base["CHANGE_SPLIT_GOVERNANCE_ENABLED"],
		"CHANGE_SPLIT_RISK_DOMAINS":       base["CHANGE_SPLIT_RISK_DOMAINS"],
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "10",
	})
	if err != nil {
		t.Fatalf("expected boundary allow, err=%v output=%s", err, allow2)
	}
	if !strings.Contains(allow2, "QUALITY_GATE_REASON=below_split_threshold") {
		t.Fatalf("unexpected boundary reason: %s", allow2)
	}

	block3, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": base["CHANGE_SPLIT_GOVERNANCE_ENABLED"],
		"CHANGE_SPLIT_RISK_DOMAINS":       base["CHANGE_SPLIT_RISK_DOMAINS"],
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "3",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "10",
	})
	if err == nil {
		t.Fatalf("expected capability threshold block, output=%s", block3)
	}
	if !strings.Contains(block3, "QUALITY_GATE_REASON=split_required_missing_plan") {
		t.Fatalf("unexpected threshold block reason: %s", block3)
	}

	block11, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": base["CHANGE_SPLIT_GOVERNANCE_ENABLED"],
		"CHANGE_SPLIT_RISK_DOMAINS":       base["CHANGE_SPLIT_RISK_DOMAINS"],
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "11",
	})
	if err == nil {
		t.Fatalf("expected increment threshold block, output=%s", block11)
	}
	if !strings.Contains(block11, "QUALITY_GATE_REASON=split_required_missing_plan") {
		t.Fatalf("unexpected item threshold block reason: %s", block11)
	}
}

func TestChangeSplitGovernanceAcceptanceBlockAndSuggestion(t *testing.T) {
	out, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "4",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "12",
		"CHANGE_SPLIT_RISK_DOMAINS":       "blocking",
	})
	if err == nil {
		t.Fatalf("expected block when threshold exceeded without plan, output=%s", out)
	}
	if !strings.Contains(out, "QUALITY_GATE_RESULT=block") || !strings.Contains(out, "QUALITY_GATE_REASON=split_required_missing_plan") {
		t.Fatalf("unexpected acceptance output: %s", out)
	}
	if !strings.Contains(out, "QUALITY_GATE_FIX_HINT=provide split plan") {
		t.Fatalf("expected split suggestion fix hint: %s", out)
	}
}

func TestChangeSplitGovernanceMixedRiskAndExceptions(t *testing.T) {
	withoutApproval, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "6",
		"CHANGE_SPLIT_RISK_DOMAINS":       "blocking,operational",
	})
	if err == nil {
		t.Fatalf("expected mixed risk without approval to block, output=%s", withoutApproval)
	}
	if !strings.Contains(withoutApproval, "QUALITY_GATE_REASON=mixed_risk_domains_without_exception") {
		t.Fatalf("unexpected mixed risk reason: %s", withoutApproval)
	}

	missingFields, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "6",
		"CHANGE_SPLIT_RISK_DOMAINS":       "blocking,operational",
		"CHANGE_SPLIT_EXCEPTION_APPROVED": "true",
	})
	if err == nil {
		t.Fatalf("expected missing exception fields to block, output=%s", missingFields)
	}
	if !strings.Contains(missingFields, "QUALITY_GATE_REASON=exception_approval_fields_missing") {
		t.Fatalf("unexpected missing-field reason: %s", missingFields)
	}

	approved, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED":  "true",
		"CHANGE_SPLIT_CAPABILITY_COUNT":    "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":     "6",
		"CHANGE_SPLIT_RISK_DOMAINS":        "blocking,operational",
		"CHANGE_SPLIT_EXCEPTION_APPROVED":  "true",
		"CHANGE_SPLIT_EXCEPTION_APPROVER":  "oncall-a",
		"CHANGE_SPLIT_EXCEPTION_REASON":    "incident mitigation",
		"CHANGE_SPLIT_EXCEPTION_TIMESTAMP": "2026-03-05T02:00:00Z",
		"CHANGE_SPLIT_EXCEPTION_TRACE_KEY": "trace-123",
	})
	if err != nil {
		t.Fatalf("expected approved exception to pass: %v output=%s", err, approved)
	}
	if !strings.Contains(approved, "QUALITY_GATE_RESULT=allow") {
		t.Fatalf("expected allow with exception: %s", approved)
	}
}

func TestChangeSplitGovernanceArchiveChecklistAndIdempotency(t *testing.T) {
	checklistMissing, err := runChangeSplitGovernance(t, map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED": "true",
		"CHANGE_SPLIT_STAGE":              "archive",
		"CHANGE_SPLIT_CAPABILITY_COUNT":   "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":    "6",
	})
	if err == nil {
		t.Fatalf("expected checklist missing to block, output=%s", checklistMissing)
	}
	if !strings.Contains(checklistMissing, "QUALITY_GATE_REASON=pre_archive_checklist_incomplete") {
		t.Fatalf("unexpected checklist reason: %s", checklistMissing)
	}

	stateFile := filepath.Join(t.TempDir(), "idempotency.log")
	base := map[string]string{
		"CHANGE_SPLIT_GOVERNANCE_ENABLED":     "true",
		"CHANGE_SPLIT_STAGE":                  "archive",
		"CHANGE_SPLIT_CAPABILITY_COUNT":       "2",
		"CHANGE_SPLIT_INCREMENT_ITEMS":        "6",
		"CHANGE_SPLIT_CHECK_SCOPE_COMPLEXITY": "low",
		"CHANGE_SPLIT_CHECK_RISK_COUPLING":    "low",
		"CHANGE_SPLIT_CHECK_REVIEWABILITY":    "high",
		"CHANGE_SPLIT_CHECK_ROLLBACK_IMPACT":  "low",
		"CHANGE_SPLIT_IDEMPOTENCY_FILE":       stateFile,
		"CHANGE_SPLIT_SUBMISSION_KEY":         "submission-1",
	}
	first, err := runChangeSplitGovernance(t, base)
	if err != nil {
		t.Fatalf("expected first archive submission pass: %v output=%s", err, first)
	}
	second, err := runChangeSplitGovernance(t, base)
	if err != nil {
		t.Fatalf("expected idempotent replay pass: %v output=%s", err, second)
	}
	if !strings.Contains(second, "QUALITY_GATE_REASON=idempotent_replay") || !strings.Contains(second, "QUALITY_GATE_GOV_IDEMPOTENT=true") {
		t.Fatalf("unexpected idempotent replay output: %s", second)
	}
}
