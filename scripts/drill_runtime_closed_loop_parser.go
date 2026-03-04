package scripts

import "fmt"

func NormalizeGateOutcome(outcome string) string {
	switch outcome {
	case "allow", "block", "degrade":
		return outcome
	default:
		return "degrade"
	}
}

func ParseGateOutcome(phase, blockReason, l2Result, l2Decision string) string {
	if blockReason != "" || phase == "Blocked" {
		return "block"
	}
	if phase == "L3" || l2Result == "degraded" {
		return "degrade"
	}
	if l2Decision == "no-healthy-candidate" || l2Decision == "dependency-validation-failed" {
		return "degrade"
	}
	if phase == "Completed" || l2Result == "success" || l2Result == "skipped" {
		return "allow"
	}
	return NormalizeGateOutcome(phase)
}

func ValidatePrecommitCIConsistency(precommitOutcome, ciOutcome string) error {
	if precommitOutcome == "" || ciOutcome == "" {
		return fmt.Errorf("missing gate outcome: precommit=%q ci=%q", precommitOutcome, ciOutcome)
	}
	pre := NormalizeGateOutcome(precommitOutcome)
	ci := NormalizeGateOutcome(ciOutcome)
	if pre != ci {
		return fmt.Errorf("gate outcome mismatch: precommit=%s ci=%s", pre, ci)
	}
	return nil
}

func ValidateOutcomeCoverage(outcomes []string) error {
	seen := map[string]bool{"allow": false, "block": false, "degrade": false}
	for _, outcome := range outcomes {
		normalized := NormalizeGateOutcome(outcome)
		if _, ok := seen[normalized]; ok {
			seen[normalized] = true
		}
	}
	for _, expected := range []string{"allow", "block", "degrade"} {
		if !seen[expected] {
			return fmt.Errorf("missing outcome %s", expected)
		}
	}
	return nil
}
