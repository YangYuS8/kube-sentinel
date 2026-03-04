package safety

import (
	"fmt"
	"os"
	"time"

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

type SLORolloutLayer struct {
	Name                    string    `json:"name" yaml:"name"`
	Environment             string    `json:"environment" yaml:"environment"`
	Namespaces              []string  `json:"namespaces" yaml:"namespaces"`
	StableWindowPassed      bool      `json:"stableWindowPassed" yaml:"stableWindowPassed"`
	RollbackConditionActive bool      `json:"rollbackConditionActive" yaml:"rollbackConditionActive"`
	EnteredAt               time.Time `json:"enteredAt" yaml:"enteredAt"`
}

type SLOThresholdSnapshot struct {
	DegradeThresholdPercent int       `json:"degradeThresholdPercent" yaml:"degradeThresholdPercent"`
	BlockThresholdPercent   int       `json:"blockThresholdPercent" yaml:"blockThresholdPercent"`
	ApprovedBy              string    `json:"approvedBy" yaml:"approvedBy"`
	ApprovedAt              time.Time `json:"approvedAt" yaml:"approvedAt"`
	Reason                  string    `json:"reason" yaml:"reason"`
}

type SLOThresholdChangeRequest struct {
	TargetObject            string `json:"targetObject" yaml:"targetObject"`
	DegradeThresholdPercent int    `json:"degradeThresholdPercent" yaml:"degradeThresholdPercent"`
	BlockThresholdPercent   int    `json:"blockThresholdPercent" yaml:"blockThresholdPercent"`
	Approver                string `json:"approver" yaml:"approver"`
	Reason                  string `json:"reason" yaml:"reason"`
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

func ValidateRolloutLayers(layers []SLORolloutLayer) error {
	if len(layers) == 0 {
		return fmt.Errorf("at least one rollout layer is required")
	}
	for index, layer := range layers {
		if layer.Name == "" {
			return fmt.Errorf("rollout layer name is required")
		}
		if len(layer.Namespaces) == 0 {
			return fmt.Errorf("rollout layer %s must define at least one namespace", layer.Name)
		}
		if index > 0 {
			previous := layers[index-1]
			if !previous.StableWindowPassed {
				return fmt.Errorf("cannot enter layer %s before previous layer %s passes stable window", layer.Name, previous.Name)
			}
			if previous.RollbackConditionActive {
				return fmt.Errorf("cannot enter layer %s while previous layer %s rollback condition is active", layer.Name, previous.Name)
			}
		}
	}
	return nil
}

func ShouldImmediateRollback(layer SLORolloutLayer) bool {
	return layer.RollbackConditionActive
}

func ApplyThresholdChange(current SLOGovernancePolicy, request SLOThresholdChangeRequest, observedAt map[string]time.Time, now time.Time) (SLOGovernancePolicy, SLOThresholdSnapshot, error) {
	if request.TargetObject == "" {
		return current, SLOThresholdSnapshot{}, fmt.Errorf("target object is required")
	}
	if request.Approver == "" {
		return current, SLOThresholdSnapshot{}, fmt.Errorf("threshold change requires approval")
	}
	if request.DegradeThresholdPercent < 1 || request.DegradeThresholdPercent > 100 {
		return current, SLOThresholdSnapshot{}, fmt.Errorf("degrade threshold percent must be between 1 and 100")
	}
	if request.BlockThresholdPercent < 1 || request.BlockThresholdPercent > 100 {
		return current, SLOThresholdSnapshot{}, fmt.Errorf("block threshold percent must be between 1 and 100")
	}
	if request.DegradeThresholdPercent >= request.BlockThresholdPercent {
		return current, SLOThresholdSnapshot{}, fmt.Errorf("degrade threshold percent must be less than block threshold percent")
	}
	if observedAt != nil {
		if last, ok := observedAt[request.TargetObject]; ok {
			nextAllowed := last.Add(time.Duration(current.SampleWindowMinutes) * time.Minute)
			if now.Before(nextAllowed) {
				return current, SLOThresholdSnapshot{}, fmt.Errorf("observation window active for %s, retry after %s", request.TargetObject, nextAllowed.Format(time.RFC3339))
			}
		}
	}

	next := current
	next.DegradeThresholdPercent = request.DegradeThresholdPercent
	next.BlockThresholdPercent = request.BlockThresholdPercent
	if err := next.Validate(); err != nil {
		return current, SLOThresholdSnapshot{}, err
	}

	snapshot := SLOThresholdSnapshot{
		DegradeThresholdPercent: current.DegradeThresholdPercent,
		BlockThresholdPercent:   current.BlockThresholdPercent,
		ApprovedBy:              request.Approver,
		ApprovedAt:              now,
		Reason:                  request.Reason,
	}

	if observedAt != nil {
		observedAt[request.TargetObject] = now
	}

	return next, snapshot, nil
}

func RollbackThresholdChange(current SLOGovernancePolicy, snapshot SLOThresholdSnapshot) (SLOGovernancePolicy, error) {
	rolledBack := current
	rolledBack.DegradeThresholdPercent = snapshot.DegradeThresholdPercent
	rolledBack.BlockThresholdPercent = snapshot.BlockThresholdPercent
	if err := rolledBack.Validate(); err != nil {
		return current, err
	}
	return rolledBack, nil
}
