package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type HealingPhase string

const (
	PhasePending   HealingPhase = "Pending"
	PhaseL1        HealingPhase = "L1"
	PhaseL2        HealingPhase = "L2"
	PhaseL3        HealingPhase = "L3"
	PhaseCompleted HealingPhase = "Completed"
	PhaseBlocked   HealingPhase = "Blocked"
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

type HealingRequestSpec struct {
	Workload           WorkloadRef          `json:"workload"`
	MaintenanceWindows []string             `json:"maintenanceWindows,omitempty"`
	RateLimit          RateLimitSpec        `json:"rateLimit"`
	BlastRadius        BlastRadiusSpec      `json:"blastRadius"`
	CircuitBreaker     CircuitBreakerSpec   `json:"circuitBreaker"`
	HealthyRevision    HealthyRevisionSpec  `json:"healthyRevision"`
}

type CircuitBreakerStatus struct {
	ObjectOpen bool   `json:"objectOpen,omitempty"`
	DomainOpen bool   `json:"domainOpen,omitempty"`
	OpenReason string `json:"openReason,omitempty"`
}

type HealingRequestStatus struct {
	Phase               HealingPhase         `json:"phase,omitempty"`
	LastAction          string               `json:"lastAction,omitempty"`
	LastError           string               `json:"lastError,omitempty"`
	LastHealthyRevision string               `json:"lastHealthyRevision,omitempty"`
	AuditRef            string               `json:"auditRef,omitempty"`
	ObservedGeneration  int64                `json:"observedGeneration,omitempty"`
	CircuitBreaker      CircuitBreakerStatus `json:"circuitBreaker,omitempty"`
	Conditions          []metav1.Condition   `json:"conditions,omitempty"`
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
}

func (r *HealingRequest) Validate() error {
	if r.Spec.Workload.Kind != "Deployment" {
		return fmt.Errorf("unsupported workload kind %q: only Deployment is allowed in v1alpha1", r.Spec.Workload.Kind)
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
	if r.Spec.BlastRadius.MaxPodPercentage < 1 || r.Spec.BlastRadius.MaxPodPercentage > 100 {
		return fmt.Errorf("blastRadius.maxPodPercentage must be between 1 and 100")
	}
	if r.Spec.CircuitBreaker.ObjectFailureThreshold < 1 || r.Spec.CircuitBreaker.DomainFailureThreshold < 1 {
		return fmt.Errorf("circuitBreaker thresholds must be >= 1")
	}
	if r.Spec.CircuitBreaker.Scope != BreakerScopeNamespace && r.Spec.CircuitBreaker.Scope != BreakerScopeGlobal {
		return fmt.Errorf("circuitBreaker.scope must be Namespace or Global")
	}
	return nil
}
