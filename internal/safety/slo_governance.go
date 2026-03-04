package safety

import (
	"fmt"
	"os"

	"sigs.k8s.io/yaml"
)

type SLOGovernancePolicy struct {
	TargetSuccessRatePercent int `json:"targetSuccessRatePercent" yaml:"targetSuccessRatePercent"`
	SampleWindowMinutes      int `json:"sampleWindowMinutes" yaml:"sampleWindowMinutes"`
	ErrorBudgetPercent       int `json:"errorBudgetPercent" yaml:"errorBudgetPercent"`
	DegradeThresholdPercent  int `json:"degradeThresholdPercent" yaml:"degradeThresholdPercent"`
	BlockThresholdPercent    int `json:"blockThresholdPercent" yaml:"blockThresholdPercent"`
}

type SLOEvaluation struct {
	Outcome      GateOutcome `json:"outcome"`
	BudgetStatus string      `json:"budgetStatus"`
}

type IncidentResponsePlan struct {
	Severity               string `json:"severity"`
	RecoveryCondition      string `json:"recoveryCondition"`
	Runbook                string `json:"runbook"`
	ManualApprovalRequired bool   `json:"manualApprovalRequired"`
}

func DefaultSLOGovernancePolicy() SLOGovernancePolicy {
	return SLOGovernancePolicy{
		TargetSuccessRatePercent: 99,
		SampleWindowMinutes:      10,
		ErrorBudgetPercent:       5,
		DegradeThresholdPercent:  60,
		BlockThresholdPercent:    90,
	}
}

func (policy SLOGovernancePolicy) WithDefaults() SLOGovernancePolicy {
	defaults := DefaultSLOGovernancePolicy()
	if policy.TargetSuccessRatePercent == 0 {
		policy.TargetSuccessRatePercent = defaults.TargetSuccessRatePercent
	}
	if policy.SampleWindowMinutes == 0 {
		policy.SampleWindowMinutes = defaults.SampleWindowMinutes
	}
	if policy.ErrorBudgetPercent == 0 {
		policy.ErrorBudgetPercent = defaults.ErrorBudgetPercent
	}
	if policy.DegradeThresholdPercent == 0 {
		policy.DegradeThresholdPercent = defaults.DegradeThresholdPercent
	}
	if policy.BlockThresholdPercent == 0 {
		policy.BlockThresholdPercent = defaults.BlockThresholdPercent
	}
	return policy
}

func (policy SLOGovernancePolicy) Validate() error {
	if policy.TargetSuccessRatePercent < 1 || policy.TargetSuccessRatePercent > 100 {
		return fmt.Errorf("target success rate must be between 1 and 100")
	}
	if policy.SampleWindowMinutes < 1 || policy.SampleWindowMinutes > 1440 {
		return fmt.Errorf("sample window minutes must be between 1 and 1440")
	}
	if policy.ErrorBudgetPercent < 1 || policy.ErrorBudgetPercent > 100 {
		return fmt.Errorf("error budget percent must be between 1 and 100")
	}
	if policy.DegradeThresholdPercent < 1 || policy.DegradeThresholdPercent > 100 {
		return fmt.Errorf("degrade threshold percent must be between 1 and 100")
	}
	if policy.BlockThresholdPercent < 1 || policy.BlockThresholdPercent > 100 {
		return fmt.Errorf("block threshold percent must be between 1 and 100")
	}
	if policy.DegradeThresholdPercent >= policy.BlockThresholdPercent {
		return fmt.Errorf("degrade threshold percent must be less than block threshold percent")
	}
	return nil
}

func LoadSLOGovernancePolicy(filePath string) (SLOGovernancePolicy, error) {
	if filePath == "" {
		policy := DefaultSLOGovernancePolicy()
		return policy, nil
	}
	raw, err := os.ReadFile(filePath)
	if err != nil {
		return SLOGovernancePolicy{}, err
	}
	policy := SLOGovernancePolicy{}
	if err := yaml.Unmarshal(raw, &policy); err != nil {
		return SLOGovernancePolicy{}, err
	}
	policy = policy.WithDefaults()
	if err := policy.Validate(); err != nil {
		return SLOGovernancePolicy{}, err
	}
	return policy, nil
}

func EvaluateSLOBudget(policy SLOGovernancePolicy, budgetConsumedPercent float64) SLOEvaluation {
	if budgetConsumedPercent >= float64(policy.BlockThresholdPercent) {
		return SLOEvaluation{Outcome: GateOutcomeBlock, BudgetStatus: "exhausted"}
	}
	if budgetConsumedPercent >= float64(policy.DegradeThresholdPercent) {
		return SLOEvaluation{Outcome: GateOutcomeDegrade, BudgetStatus: "warning"}
	}
	return SLOEvaluation{Outcome: GateOutcomeAllow, BudgetStatus: "healthy"}
}

func MapIncidentResponsePlan(outcome GateOutcome) IncidentResponsePlan {
	switch outcome {
	case GateOutcomeAllow:
		return IncidentResponsePlan{
			Severity:               "info",
			RecoveryCondition:      "maintain target success rate and healthy budget",
			Runbook:                "runbook://runtime-observation",
			ManualApprovalRequired: false,
		}
	case GateOutcomeDegrade:
		return IncidentResponsePlan{
			Severity:               "warning",
			RecoveryCondition:      "recover budget below degrade threshold and validate gate evidence",
			Runbook:                "runbook://runtime-degrade-recovery",
			ManualApprovalRequired: false,
		}
	default:
		return IncidentResponsePlan{
			Severity:               "critical",
			RecoveryCondition:      "manual approval after budget recovery and incident review",
			Runbook:                "runbook://runtime-block-rollback",
			ManualApprovalRequired: true,
		}
	}
}
