package scripts

import "testing"

func TestParseGateOutcome(t *testing.T) {
	tests := []struct {
		name       string
		phase      string
		block      string
		l2Result   string
		l2Decision string
		expected   string
	}{
		{name: "blocked by reason", phase: "Completed", block: "statefulset_readonly", expected: "block"},
		{name: "phase blocked", phase: "Blocked", expected: "block"},
		{name: "phase l3", phase: "L3", expected: "degrade"},
		{name: "degraded result", phase: "L2", l2Result: "degraded", expected: "degrade"},
		{name: "completed success", phase: "Completed", l2Result: "success", expected: "allow"},
		{name: "fallback unknown", phase: "PendingVerify", expected: "degrade"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseGateOutcome(test.phase, test.block, test.l2Result, test.l2Decision); got != test.expected {
				t.Fatalf("expected %s, got %s", test.expected, got)
			}
		})
	}
}

func TestValidateOutcomeCoverage(t *testing.T) {
	if err := ValidateOutcomeCoverage([]string{"allow", "block", "degrade"}); err != nil {
		t.Fatalf("expected full coverage: %v", err)
	}
	if err := ValidateOutcomeCoverage([]string{"allow", "block"}); err == nil {
		t.Fatalf("expected missing degrade to fail")
	}
}
