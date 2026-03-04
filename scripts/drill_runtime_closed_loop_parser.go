package scripts

import "fmt"

type IncidentAction struct {
	Level             string
	RecoveryCondition string
	Runbook           string
}

type RolloutStageEvidence struct {
	CanaryStable     bool
	RollbackHit      bool
	TuningApproved   bool
	RecoveryObserved bool
}

type PostmortemEvidence struct {
	BreachReason      string
	MitigationAction  string
	ThresholdDecision string
	ObservationPlan   string
}

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

func ValidateGateSLOConsistency(gateOutcome, sloOutcome string) error {
	gate := NormalizeGateOutcome(gateOutcome)
	slo := NormalizeGateOutcome(sloOutcome)
	if gate != slo {
		return fmt.Errorf("gate/slo semantic mismatch: gate=%s slo=%s", gate, slo)
	}
	return nil
}

func MapIncidentAction(outcome string) IncidentAction {
	normalized := NormalizeGateOutcome(outcome)
	switch normalized {
	case "allow":
		return IncidentAction{
			Level:             "info",
			RecoveryCondition: "maintain_target_and_observe",
			Runbook:           "runbook://runtime-observation",
		}
	case "degrade":
		return IncidentAction{
			Level:             "warning",
			RecoveryCondition: "recover_budget_below_degrade_threshold",
			Runbook:           "runbook://runtime-degrade-recovery",
		}
	default:
		return IncidentAction{
			Level:             "critical",
			RecoveryCondition: "manual_approval_after_incident_review",
			Runbook:           "runbook://runtime-block-rollback",
		}
	}
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

func ValidateRolloutStageProgression(evidence RolloutStageEvidence) error {
	if !evidence.CanaryStable {
		return fmt.Errorf("missing canary stable evidence")
	}
	if !evidence.RollbackHit {
		return fmt.Errorf("missing rollback trigger evidence")
	}
	if !evidence.TuningApproved {
		return fmt.Errorf("missing threshold tuning approval evidence")
	}
	if !evidence.RecoveryObserved {
		return fmt.Errorf("missing recovery observation evidence")
	}
	return nil
}

func ValidatePostmortemEvidence(evidence PostmortemEvidence) error {
	if evidence.BreachReason == "" {
		return fmt.Errorf("missing postmortem breachReason")
	}
	if evidence.MitigationAction == "" {
		return fmt.Errorf("missing postmortem mitigationAction")
	}
	if evidence.ThresholdDecision == "" {
		return fmt.Errorf("missing postmortem thresholdDecision")
	}
	if evidence.ObservationPlan == "" {
		return fmt.Errorf("missing postmortem observationPlan")
	}
	return nil
}
