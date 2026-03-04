package healing

import (
	"fmt"
	"sort"
)

type L2RollbackWindow struct {
	WindowSize int
	Failures   int
	Degrades   int
}

type L2GateImpact struct {
	Allow                   bool
	Reason                  string
	FailurePercent          float64
	CombinedRiskPercent     float64
	RecommendedAction       string
	TriggerConservativeMode bool
}

func SelectLatestHealthyRevision(records []RevisionRecord) (RevisionRecord, error) {
	if len(records) == 0 {
		return RevisionRecord{}, fmt.Errorf("no revisions found")
	}
	sorted := append([]RevisionRecord(nil), records...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UnixTime > sorted[j].UnixTime
	})
	for _, rec := range sorted {
		if rec.Healthy {
			return rec, nil
		}
	}
	return RevisionRecord{}, fmt.Errorf("no healthy revision available")
}

func EvaluateL2RollbackGate(window L2RollbackWindow, maxFailurePercent int) L2GateImpact {
	if window.WindowSize < 0 {
		return L2GateImpact{
			Allow:                   false,
			Reason:                  "l2 rollback gate blocked: invalid continuous window",
			RecommendedAction:       "fallback to conservative mode and check l2 rollback sample window",
			TriggerConservativeMode: true,
		}
	}
	if window.WindowSize == 0 {
		return L2GateImpact{
			Allow:               true,
			Reason:              "l2 rollback gate allow: no historical samples yet",
			FailurePercent:      0,
			CombinedRiskPercent: 0,
			RecommendedAction:   "continue rollout and collect l2 rollback samples",
		}
	}
	if maxFailurePercent < 1 || maxFailurePercent > 100 {
		return L2GateImpact{
			Allow:                   false,
			Reason:                  "l2 rollback gate blocked: invalid failure threshold",
			RecommendedAction:       "fallback to conservative mode and correct l2 rollback threshold",
			TriggerConservativeMode: true,
		}
	}
	if window.Failures < 0 {
		window.Failures = 0
	}
	if window.Degrades < 0 {
		window.Degrades = 0
	}
	failurePercent := float64(window.Failures*100) / float64(window.WindowSize)
	combinedRiskPercent := float64((window.Failures+window.Degrades)*100) / float64(window.WindowSize)
	if failurePercent > float64(maxFailurePercent) {
		return L2GateImpact{
			Allow:                   false,
			Reason:                  fmt.Sprintf("l2 rollback gate blocked: failure rate %.1f%% > %d%%", failurePercent, maxFailurePercent),
			FailurePercent:          failurePercent,
			CombinedRiskPercent:     combinedRiskPercent,
			RecommendedAction:       "degrade to read-only, emit rollback alert, and verify snapshot recovery evidence",
			TriggerConservativeMode: true,
		}
	}
	return L2GateImpact{
		Allow:               true,
		Reason:              fmt.Sprintf("l2 rollback gate allow: failure rate %.1f%% <= %d%%", failurePercent, maxFailurePercent),
		FailurePercent:      failurePercent,
		CombinedRiskPercent: combinedRiskPercent,
		RecommendedAction:   "continue rollout and observe l2 rollback stability",
	}
}
