package healing

import "testing"

func TestSelectLatestHealthyRevision(t *testing.T) {
	recs := []RevisionRecord{
		{Revision: "1", UnixTime: 100, Healthy: true},
		{Revision: "2", UnixTime: 200, Healthy: false},
		{Revision: "3", UnixTime: 300, Healthy: true},
	}
	r, err := SelectLatestHealthyRevision(recs)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if r.Revision != "3" {
		t.Fatalf("want latest healthy revision 3, got %s", r.Revision)
	}
}

func TestSelectLatestHealthyRevisionNoHealthy(t *testing.T) {
	recs := []RevisionRecord{{Revision: "1", UnixTime: 1, Healthy: false}}
	if _, err := SelectLatestHealthyRevision(recs); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEvaluateL2RollbackGateAllowWithinThreshold(t *testing.T) {
	impact := EvaluateL2RollbackGate(L2RollbackWindow{WindowSize: 10, Failures: 2, Degrades: 1}, 30)
	if !impact.Allow {
		t.Fatalf("expected l2 gate allow")
	}
	if impact.FailurePercent != 20 {
		t.Fatalf("expected failure percent 20, got %.1f", impact.FailurePercent)
	}
}

func TestEvaluateL2RollbackGateBlockBeyondThreshold(t *testing.T) {
	impact := EvaluateL2RollbackGate(L2RollbackWindow{WindowSize: 10, Failures: 4, Degrades: 1}, 30)
	if impact.Allow {
		t.Fatalf("expected l2 gate blocked")
	}
	if !impact.TriggerConservativeMode {
		t.Fatalf("expected conservative mode trigger")
	}
}

func TestEvaluateL2RollbackGateInvalidInput(t *testing.T) {
	impact := EvaluateL2RollbackGate(L2RollbackWindow{WindowSize: -1, Failures: 1}, 30)
	if impact.Allow {
		t.Fatalf("expected invalid window to block")
	}
	impact = EvaluateL2RollbackGate(L2RollbackWindow{WindowSize: 10, Failures: 1}, 101)
	if impact.Allow {
		t.Fatalf("expected invalid threshold to block")
	}
}

func TestEvaluateL2RollbackGateColdStartAllow(t *testing.T) {
	impact := EvaluateL2RollbackGate(L2RollbackWindow{WindowSize: 0}, 30)
	if !impact.Allow {
		t.Fatalf("expected cold-start window to allow")
	}
}
