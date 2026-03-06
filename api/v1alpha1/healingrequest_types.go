package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type HealingPhase string

const (
	PhasePending       HealingPhase = "Pending"
	PhasePendingVerify HealingPhase = "PendingVerify"
	PhaseSuppressed    HealingPhase = "Suppressed"
	PhaseL1            HealingPhase = "L1"
	PhaseL2            HealingPhase = "L2"
	PhaseL3            HealingPhase = "L3"
	PhaseCompleted     HealingPhase = "Completed"
	PhaseBlocked       HealingPhase = "Blocked"
)

type BreakerScope string

const (
	BreakerScopeNamespace BreakerScope = "Namespace"
	BreakerScopeGlobal    BreakerScope = "Global"
)

type WorkloadRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

type RateLimitSpec struct {
	MaxActions    int `json:"maxActions"`
	WindowMinutes int `json:"windowMinutes"`
}

type BlastRadiusSpec struct {
	MaxPodPercentage int `json:"maxPodPercentage"`
}

type CircuitBreakerSpec struct {
	ObjectFailureThreshold int          `json:"objectFailureThreshold"`
	DomainFailureThreshold int          `json:"domainFailureThreshold"`
	CooldownMinutes        int          `json:"cooldownMinutes"`
	Scope                  BreakerScope `json:"scope"`
}

type HealthyRevisionSpec struct {
	ObserveMinutes          int  `json:"observeMinutes"`
	RequireNoCriticalAlerts bool `json:"requireNoCriticalAlerts"`
}

type APICompatibilityClass string

const (
	APICompatibilityBackwardCompatible APICompatibilityClass = "backward-compatible"
	APICompatibilityMigrationRequired  APICompatibilityClass = "migration-required"
	APICompatibilityVersionBumpNeeded  APICompatibilityClass = "version-bump-required"
)

type APIContractRiskLevel string

const (
	APIContractRiskLow    APIContractRiskLevel = "low"
	APIContractRiskMedium APIContractRiskLevel = "medium"
	APIContractRiskHigh   APIContractRiskLevel = "high"
)

type APIContractPolicySpec struct {
	CompatibilityClass  APICompatibilityClass `json:"compatibilityClass,omitempty"`
	MigrationPlanRef    string                `json:"migrationPlanRef,omitempty"`
	VersionBumpWindow   string                `json:"versionBumpWindow,omitempty"`
	RiskLevel           APIContractRiskLevel  `json:"riskLevel,omitempty"`
	RequireStatusFields bool                  `json:"requireStatusFields,omitempty"`
}

type SoakTimePolicySpec struct {
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	DurationSec int    `json:"durationSec"`
	MinSamples  int    `json:"minSamples"`
}

type NamespaceBudgetSpec struct {
	BlockingThresholdPercent int `json:"blockingThresholdPercent"`
	MinTotalWorkloads        int `json:"minTotalWorkloads"`
	FallbackUnhealthyCount   int `json:"fallbackUnhealthyCount"`
}

type EmergencyTrySpec struct {
	Enabled     bool `json:"enabled"`
	MaxAttempts int  `json:"maxAttempts"`
}

type StatefulSetPolicySpec struct {
	Enabled                  bool     `json:"enabled,omitempty"`
	ReadOnlyOnly             bool     `json:"readOnlyOnly,omitempty"`
	ControlledActionsEnabled bool     `json:"controlledActionsEnabled,omitempty"`
	L2RollbackEnabled        bool     `json:"l2RollbackEnabled,omitempty"`
	AllowedNamespaces        []string `json:"allowedNamespaces,omitempty"`
	ApprovalAnnotation       string   `json:"approvalAnnotation,omitempty"`
	RequireEvidence          bool     `json:"requireEvidence,omitempty"`
	FreezeWindowMinutes      int      `json:"freezeWindowMinutes,omitempty"`
	L2CandidateWindowMinutes int      `json:"l2CandidateWindowMinutes,omitempty"`
	L2MaxDegradeRatePercent  int      `json:"l2MaxDegradeRatePercent,omitempty"`
}

type DeploymentPolicySpec struct {
	L2CandidateWindowMinutes int `json:"l2CandidateWindowMinutes,omitempty"`
	L2MaxDegradeRatePercent  int `json:"l2MaxDegradeRatePercent,omitempty"`
	L1SuccessRateMinPercent  int `json:"l1SuccessRateMinPercent,omitempty"`
	L2SuccessRateMinPercent  int `json:"l2SuccessRateMinPercent,omitempty"`
	L3DegradeRateMaxPercent  int `json:"l3DegradeRateMaxPercent,omitempty"`
	BlockRateMaxPercent      int `json:"blockRateMaxPercent,omitempty"`
}

type ProductionGatePolicySpec struct {
	SampleWindowMinutes      int `json:"sampleWindowMinutes,omitempty"`
	FailureRatioBlockPercent int `json:"failureRatioBlockPercent,omitempty"`
}

type SnapshotPolicySpec struct {
	Enabled                 bool `json:"enabled,omitempty"`
	RetentionMinutes        int  `json:"retentionMinutes,omitempty"`
	RestoreTimeoutSeconds   int  `json:"restoreTimeoutSeconds,omitempty"`
	MaxSnapshotsPerWorkload int  `json:"maxSnapshotsPerWorkload,omitempty"`
}

type HealingRequestSpec struct {
	Workload                 WorkloadRef              `json:"workload"`
	StatefulSetPolicy        StatefulSetPolicySpec    `json:"statefulSetPolicy,omitempty"`
	DeploymentPolicy         DeploymentPolicySpec     `json:"deploymentPolicy,omitempty"`
	SnapshotPolicy           SnapshotPolicySpec       `json:"snapshotPolicy,omitempty"`
	APIContractPolicy        APIContractPolicySpec    `json:"apiContractPolicy,omitempty"`
	ProductionGatePolicy     ProductionGatePolicySpec `json:"productionGatePolicy,omitempty"`
	MaintenanceWindows       []string                 `json:"maintenanceWindows,omitempty"`
	IdempotencyWindowMinutes int                      `json:"idempotencyWindowMinutes,omitempty"`
	RateLimit                RateLimitSpec            `json:"rateLimit"`
	BlastRadius              BlastRadiusSpec          `json:"blastRadius"`
	CircuitBreaker           CircuitBreakerSpec       `json:"circuitBreaker"`
	HealthyRevision          HealthyRevisionSpec      `json:"healthyRevision"`
	SoakTimePolicies         []SoakTimePolicySpec     `json:"soakTimePolicies,omitempty"`
	NamespaceBudget          NamespaceBudgetSpec      `json:"namespaceBudget"`
	EmergencyTry             EmergencyTrySpec         `json:"emergencyTry"`
}

type CircuitBreakerStatus struct {
	ObjectOpen            bool   `json:"objectOpen,omitempty"`
	DomainOpen            bool   `json:"domainOpen,omitempty"`
	OpenReason            string `json:"openReason,omitempty"`
	CurrentObjectFailures int    `json:"currentObjectFailures,omitempty"`
	CurrentDomainFailures int    `json:"currentDomainFailures,omitempty"`
	RecoveryAt            string `json:"recoveryAt,omitempty"`
}

type HealingRequestStatus struct {
	Phase                    HealingPhase         `json:"phase,omitempty"`
	WorkloadCapability       string               `json:"workloadCapability,omitempty"`
	StatefulSetAuthorization string               `json:"statefulSetAuthorization,omitempty"`
	StatefulSetFreezeState   string               `json:"statefulSetFreezeState,omitempty"`
	StatefulSetFreezeUntil   string               `json:"statefulSetFreezeUntil,omitempty"`
	StatefulSetFailureReason string               `json:"statefulSetFailureReason,omitempty"`
	StatefulSetL2Candidate   string               `json:"statefulSetL2Candidate,omitempty"`
	StatefulSetL2Decision    string               `json:"statefulSetL2Decision,omitempty"`
	StatefulSetL2Result      string               `json:"statefulSetL2Result,omitempty"`
	DeploymentL2Candidate    string               `json:"deploymentL2Candidate,omitempty"`
	DeploymentL2Decision     string               `json:"deploymentL2Decision,omitempty"`
	DeploymentL2Result       string               `json:"deploymentL2Result,omitempty"`
	LastSnapshotID           string               `json:"lastSnapshotId,omitempty"`
	SnapshotRestoreResult    string               `json:"snapshotRestoreResult,omitempty"`
	SnapshotFailureReason    string               `json:"snapshotFailureReason,omitempty"`
	SnapshotActiveCount      int                  `json:"snapshotActiveCount,omitempty"`
	NextRecommendation       string               `json:"nextRecommendation,omitempty"`
	BlockReasonCode          string               `json:"blockReasonCode,omitempty"`
	LastAction               string               `json:"lastAction,omitempty"`
	LastError                string               `json:"lastError,omitempty"`
	LastGateDecision         string               `json:"lastGateDecision,omitempty"`
	GateOutcome              string               `json:"gateOutcome,omitempty"`
	GateReasonCode           string               `json:"gateReasonCode,omitempty"`
	GateEvidenceComplete     bool                 `json:"gateEvidenceComplete,omitempty"`
	LastEvidenceStatus       string               `json:"lastEvidenceStatus,omitempty"`
	LastEventReason          string               `json:"lastEventReason,omitempty"`
	PendingSince             string               `json:"pendingSince,omitempty"`
	SuppressedAt             string               `json:"suppressedAt,omitempty"`
	StableSampleCount        int                  `json:"stableSampleCount,omitempty"`
	ShadowAction             string               `json:"shadowAction,omitempty"`
	NamespaceBlockRate       int                  `json:"namespaceBlockRate,omitempty"`
	EmergencyAttempts        int                  `json:"emergencyAttempts,omitempty"`
	CorrelationKey           string               `json:"correlationKey,omitempty"`
	LastHealthyRevision      string               `json:"lastHealthyRevision,omitempty"`
	AuditRef                 string               `json:"auditRef,omitempty"`
	ObservedGeneration       int64                `json:"observedGeneration,omitempty"`
	CircuitBreaker           CircuitBreakerStatus `json:"circuitBreaker,omitempty"`
	Conditions               []metav1.Condition   `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type HealingRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealingRequestSpec   `json:"spec,omitempty"`
	Status HealingRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
type HealingRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealingRequest `json:"items"`
}

func (r *HealingRequest) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := *r
	out.Spec.MaintenanceWindows = append([]string(nil), r.Spec.MaintenanceWindows...)
	out.Spec.SoakTimePolicies = append([]SoakTimePolicySpec(nil), r.Spec.SoakTimePolicies...)
	out.Status.Conditions = append([]metav1.Condition(nil), r.Status.Conditions...)
	return &out
}

func (r *HealingRequestList) DeepCopyObject() runtime.Object {
	if r == nil {
		return nil
	}
	out := *r
	out.Items = append([]HealingRequest(nil), r.Items...)
	return &out
}

func (r *HealingRequest) ApplyDefaults() {
	if r.Spec.RateLimit.MaxActions == 0 {
		r.Spec.RateLimit.MaxActions = 3
	}
	if r.Spec.RateLimit.WindowMinutes == 0 {
		r.Spec.RateLimit.WindowMinutes = 10
	}
	if r.Spec.IdempotencyWindowMinutes == 0 {
		r.Spec.IdempotencyWindowMinutes = 5
	}
	if r.Spec.BlastRadius.MaxPodPercentage == 0 {
		r.Spec.BlastRadius.MaxPodPercentage = 10
	}
	if r.Spec.CircuitBreaker.ObjectFailureThreshold == 0 {
		r.Spec.CircuitBreaker.ObjectFailureThreshold = 3
	}
	if r.Spec.CircuitBreaker.DomainFailureThreshold == 0 {
		r.Spec.CircuitBreaker.DomainFailureThreshold = 10
	}
	if r.Spec.CircuitBreaker.CooldownMinutes == 0 {
		r.Spec.CircuitBreaker.CooldownMinutes = 10
	}
	if r.Spec.CircuitBreaker.Scope == "" {
		r.Spec.CircuitBreaker.Scope = BreakerScopeNamespace
	}
	if r.Spec.HealthyRevision.ObserveMinutes == 0 {
		r.Spec.HealthyRevision.ObserveMinutes = 5
	}
	if len(r.Spec.SoakTimePolicies) == 0 {
		r.Spec.SoakTimePolicies = []SoakTimePolicySpec{
			{Category: "CrashLoopBackOff", Severity: "Critical", DurationSec: 30, MinSamples: 2},
			{Category: "OOMKilled", Severity: "High", DurationSec: 60, MinSamples: 2},
			{Category: "ProbeFailure", Severity: "Medium", DurationSec: 120, MinSamples: 3},
			{Category: "Pending", Severity: "Low", DurationSec: 300, MinSamples: 3},
		}
	}
	if r.Spec.NamespaceBudget.BlockingThresholdPercent == 0 {
		r.Spec.NamespaceBudget.BlockingThresholdPercent = 30
	}
	if r.Spec.NamespaceBudget.MinTotalWorkloads == 0 {
		r.Spec.NamespaceBudget.MinTotalWorkloads = 5
	}
	if r.Spec.NamespaceBudget.FallbackUnhealthyCount == 0 {
		r.Spec.NamespaceBudget.FallbackUnhealthyCount = 2
	}
	if r.Spec.EmergencyTry.MaxAttempts == 0 {
		r.Spec.EmergencyTry.MaxAttempts = 1
	}
	if r.Spec.StatefulSetPolicy.ApprovalAnnotation == "" {
		r.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	}
	if r.Spec.StatefulSetPolicy.FreezeWindowMinutes == 0 {
		r.Spec.StatefulSetPolicy.FreezeWindowMinutes = 10
	}
	if r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes == 0 {
		r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes = 30
	}
	if r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent == 0 {
		r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent = 10
	}
	if r.Spec.DeploymentPolicy.L2CandidateWindowMinutes == 0 {
		r.Spec.DeploymentPolicy.L2CandidateWindowMinutes = 30
	}
	if r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent == 0 {
		r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent = 10
	}
	if r.Spec.DeploymentPolicy.L1SuccessRateMinPercent == 0 {
		r.Spec.DeploymentPolicy.L1SuccessRateMinPercent = 60
	}
	if r.Spec.DeploymentPolicy.L2SuccessRateMinPercent == 0 {
		r.Spec.DeploymentPolicy.L2SuccessRateMinPercent = 50
	}
	if r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent == 0 {
		r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent = 40
	}
	if r.Spec.DeploymentPolicy.BlockRateMaxPercent == 0 {
		r.Spec.DeploymentPolicy.BlockRateMaxPercent = 30
	}
	if r.Spec.ProductionGatePolicy.SampleWindowMinutes == 0 {
		r.Spec.ProductionGatePolicy.SampleWindowMinutes = 10
	}
	if r.Spec.ProductionGatePolicy.FailureRatioBlockPercent == 0 {
		r.Spec.ProductionGatePolicy.FailureRatioBlockPercent = 30
	}
	if !r.Spec.SnapshotPolicy.Enabled {
		r.Spec.SnapshotPolicy.Enabled = true
	}
	if r.Spec.SnapshotPolicy.RetentionMinutes == 0 {
		r.Spec.SnapshotPolicy.RetentionMinutes = 60
	}
	if r.Spec.SnapshotPolicy.RestoreTimeoutSeconds == 0 {
		r.Spec.SnapshotPolicy.RestoreTimeoutSeconds = 30
	}
	if r.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload == 0 {
		r.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload = 20
	}
	if r.Spec.APIContractPolicy.CompatibilityClass == "" {
		r.Spec.APIContractPolicy.CompatibilityClass = APICompatibilityBackwardCompatible
	}
	if r.Spec.APIContractPolicy.RiskLevel == "" {
		r.Spec.APIContractPolicy.RiskLevel = APIContractRiskLow
	}
	if !r.Spec.APIContractPolicy.RequireStatusFields {
		r.Spec.APIContractPolicy.RequireStatusFields = true
	}
	if r.Spec.Workload.Kind == "StatefulSet" {
		if len(r.Spec.StatefulSetPolicy.AllowedNamespaces) == 0 {
			r.Spec.StatefulSetPolicy.AllowedNamespaces = []string{r.Spec.Workload.Namespace}
		}
		if !r.Spec.StatefulSetPolicy.Enabled {
			r.Spec.StatefulSetPolicy.Enabled = true
		}
		if !r.Spec.StatefulSetPolicy.ReadOnlyOnly && !r.Spec.StatefulSetPolicy.ControlledActionsEnabled {
			r.Spec.StatefulSetPolicy.ReadOnlyOnly = true
		}
	}
}

func (r *HealingRequest) Validate() error {
	if r.Spec.Workload.Kind != "Deployment" && r.Spec.Workload.Kind != "StatefulSet" {
		return fmt.Errorf("unsupported workload kind %q: only Deployment or StatefulSet is allowed in v1alpha1", r.Spec.Workload.Kind)
	}
	if r.Spec.Workload.Name == "" || r.Spec.Workload.Namespace == "" {
		return fmt.Errorf("workload namespace/name are required")
	}
	if r.Spec.RateLimit.MaxActions < 1 {
		return fmt.Errorf("rateLimit.maxActions must be >= 1")
	}
	if r.Spec.RateLimit.WindowMinutes < 1 {
		return fmt.Errorf("rateLimit.windowMinutes must be >= 1")
	}
	if r.Spec.IdempotencyWindowMinutes < 1 {
		return fmt.Errorf("idempotencyWindowMinutes must be >= 1")
	}
	if r.Spec.BlastRadius.MaxPodPercentage < 1 || r.Spec.BlastRadius.MaxPodPercentage > 100 {
		return fmt.Errorf("blastRadius.maxPodPercentage must be between 1 and 100")
	}
	if r.Spec.CircuitBreaker.ObjectFailureThreshold < 1 || r.Spec.CircuitBreaker.DomainFailureThreshold < 1 {
		return fmt.Errorf("circuitBreaker thresholds must be >= 1")
	}
	if r.Spec.CircuitBreaker.CooldownMinutes < 1 {
		return fmt.Errorf("circuitBreaker.cooldownMinutes must be >= 1")
	}
	if r.Spec.CircuitBreaker.Scope != BreakerScopeNamespace && r.Spec.CircuitBreaker.Scope != BreakerScopeGlobal {
		return fmt.Errorf("circuitBreaker.scope must be Namespace or Global")
	}
	if r.Spec.HealthyRevision.ObserveMinutes < 1 {
		return fmt.Errorf("healthyRevision.observeMinutes must be >= 1")
	}
	if r.Spec.NamespaceBudget.BlockingThresholdPercent < 1 || r.Spec.NamespaceBudget.BlockingThresholdPercent > 100 {
		return fmt.Errorf("namespaceBudget.blockingThresholdPercent must be between 1 and 100")
	}
	if r.Spec.NamespaceBudget.MinTotalWorkloads < 1 {
		return fmt.Errorf("namespaceBudget.minTotalWorkloads must be >= 1")
	}
	if r.Spec.NamespaceBudget.FallbackUnhealthyCount < 1 {
		return fmt.Errorf("namespaceBudget.fallbackUnhealthyCount must be >= 1")
	}
	if r.Spec.EmergencyTry.MaxAttempts < 1 {
		return fmt.Errorf("emergencyTry.maxAttempts must be >= 1")
	}
	if r.Spec.StatefulSetPolicy.FreezeWindowMinutes < 1 {
		return fmt.Errorf("statefulSetPolicy.freezeWindowMinutes must be >= 1")
	}
	if r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes < 1 {
		return fmt.Errorf("statefulSetPolicy.l2CandidateWindowMinutes must be >= 1")
	}
	if r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent < 1 || r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent > 100 {
		return fmt.Errorf("statefulSetPolicy.l2MaxDegradeRatePercent must be between 1 and 100")
	}
	if r.Spec.DeploymentPolicy.L2CandidateWindowMinutes < 1 {
		return fmt.Errorf("deploymentPolicy.l2CandidateWindowMinutes must be >= 1")
	}
	if r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent < 1 || r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent > 100 {
		return fmt.Errorf("deploymentPolicy.l2MaxDegradeRatePercent must be between 1 and 100")
	}
	if r.Spec.DeploymentPolicy.L1SuccessRateMinPercent < 1 || r.Spec.DeploymentPolicy.L1SuccessRateMinPercent > 100 {
		return fmt.Errorf("deploymentPolicy.l1SuccessRateMinPercent must be between 1 and 100")
	}
	if r.Spec.DeploymentPolicy.L2SuccessRateMinPercent < 1 || r.Spec.DeploymentPolicy.L2SuccessRateMinPercent > 100 {
		return fmt.Errorf("deploymentPolicy.l2SuccessRateMinPercent must be between 1 and 100")
	}
	if r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent < 1 || r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent > 100 {
		return fmt.Errorf("deploymentPolicy.l3DegradeRateMaxPercent must be between 1 and 100")
	}
	if r.Spec.DeploymentPolicy.BlockRateMaxPercent < 1 || r.Spec.DeploymentPolicy.BlockRateMaxPercent > 100 {
		return fmt.Errorf("deploymentPolicy.blockRateMaxPercent must be between 1 and 100")
	}
	if r.Spec.ProductionGatePolicy.SampleWindowMinutes < 1 {
		return fmt.Errorf("productionGatePolicy.sampleWindowMinutes must be >= 1")
	}
	if r.Spec.ProductionGatePolicy.FailureRatioBlockPercent < 1 || r.Spec.ProductionGatePolicy.FailureRatioBlockPercent > 100 {
		return fmt.Errorf("productionGatePolicy.failureRatioBlockPercent must be between 1 and 100")
	}
	if r.Spec.SnapshotPolicy.RetentionMinutes < 1 {
		return fmt.Errorf("snapshotPolicy.retentionMinutes must be >= 1")
	}
	if r.Spec.SnapshotPolicy.RestoreTimeoutSeconds < 1 {
		return fmt.Errorf("snapshotPolicy.restoreTimeoutSeconds must be >= 1")
	}
	if r.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload < 1 {
		return fmt.Errorf("snapshotPolicy.maxSnapshotsPerWorkload must be >= 1")
	}
	if r.Spec.APIContractPolicy.CompatibilityClass != APICompatibilityBackwardCompatible &&
		r.Spec.APIContractPolicy.CompatibilityClass != APICompatibilityMigrationRequired &&
		r.Spec.APIContractPolicy.CompatibilityClass != APICompatibilityVersionBumpNeeded {
		return fmt.Errorf("apiContractPolicy.compatibilityClass must be one of backward-compatible, migration-required, version-bump-required")
	}
	if r.Spec.APIContractPolicy.RiskLevel != APIContractRiskLow &&
		r.Spec.APIContractPolicy.RiskLevel != APIContractRiskMedium &&
		r.Spec.APIContractPolicy.RiskLevel != APIContractRiskHigh {
		return fmt.Errorf("apiContractPolicy.riskLevel must be one of low, medium, high")
	}
	if r.Spec.APIContractPolicy.CompatibilityClass == APICompatibilityMigrationRequired && r.Spec.APIContractPolicy.MigrationPlanRef == "" {
		return fmt.Errorf("apiContractPolicy.migrationPlanRef is required when compatibilityClass is migration-required")
	}
	if r.Spec.APIContractPolicy.CompatibilityClass == APICompatibilityVersionBumpNeeded && r.Spec.APIContractPolicy.VersionBumpWindow == "" {
		return fmt.Errorf("apiContractPolicy.versionBumpWindow is required when compatibilityClass is version-bump-required")
	}
	if r.Spec.Workload.Kind == "StatefulSet" {
		if len(r.Spec.StatefulSetPolicy.AllowedNamespaces) == 0 {
			return fmt.Errorf("statefulSetPolicy.allowedNamespaces must not be empty for StatefulSet")
		}
		if r.Spec.StatefulSetPolicy.ApprovalAnnotation == "" {
			return fmt.Errorf("statefulSetPolicy.approvalAnnotation is required for StatefulSet")
		}
	}
	for _, policy := range r.Spec.SoakTimePolicies {
		if policy.DurationSec < 1 {
			return fmt.Errorf("soakTimePolicies.durationSec must be >= 1")
		}
		if policy.MinSamples < 1 {
			return fmt.Errorf("soakTimePolicies.minSamples must be >= 1")
		}
	}
	return nil
}

func (r *HealingRequest) ValidateAPIContractRequirements() error {
	if err := r.Validate(); err != nil {
		return err
	}
	if r.Spec.APIContractPolicy.RequireStatusFields {
		if err := r.Status.ValidateContractSemantics(); err != nil {
			return err
		}
	}
	return nil
}

func (status HealingRequestStatus) ValidateContractSemantics() error {
	if status.Phase == "" {
		return fmt.Errorf("status.phase is required for api contract semantics")
	}
	if status.LastAction == "" {
		return fmt.Errorf("status.lastAction is required for api contract semantics")
	}
	if status.LastGateDecision == "" {
		return fmt.Errorf("status.lastGateDecision is required for api contract semantics")
	}
	if status.NextRecommendation == "" {
		return fmt.Errorf("status.nextRecommendation is required for api contract semantics")
	}
	if status.Phase == PhaseBlocked || status.Phase == PhaseL3 {
		if status.BlockReasonCode == "" && status.LastError == "" {
			return fmt.Errorf("status.failureReason is required for blocked/degraded semantics")
		}
	}
	return nil
}
