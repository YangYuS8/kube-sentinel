package scripts

import "fmt"

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
	return "degrade"
}

func ValidateOutcomeCoverage(outcomes []string) error {
	seen := map[string]bool{"allow": false, "block": false, "degrade": false}
	for _, outcome := range outcomes {
		if _, ok := seen[outcome]; ok {
			seen[outcome] = true
		}
	}
	for _, expected := range []string{"allow", "block", "degrade"} {
		if !seen[expected] {
			return fmt.Errorf("missing outcome %s", expected)
		}
	}
	return nil
}
