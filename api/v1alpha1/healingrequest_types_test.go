package v1alpha1

import "testing"

func baseRequest() *HealingRequest {
	return &HealingRequest{Spec: HealingRequestSpec{Workload: WorkloadRef{Kind: "Deployment", Namespace: "default", Name: "app"}}}
}

func TestApplyDefaults(t *testing.T) {
	r := baseRequest()
	r.ApplyDefaults()
	if r.Spec.RateLimit.MaxActions != 3 || r.Spec.RateLimit.WindowMinutes != 10 {
		t.Fatalf("rate limit defaults not applied")
	}
	if r.Spec.IdempotencyWindowMinutes != 5 {
		t.Fatalf("idempotency window default not applied")
	}
	if r.Spec.BlastRadius.MaxPodPercentage != 10 {
		t.Fatalf("blast radius default not applied")
	}
	if r.Spec.CircuitBreaker.Scope != BreakerScopeNamespace {
		t.Fatalf("circuit breaker scope default not applied")
	}
	if len(r.Spec.SoakTimePolicies) == 0 {
		t.Fatalf("soak time policies default not applied")
	}
	if r.Spec.NamespaceBudget.BlockingThresholdPercent != 30 || r.Spec.NamespaceBudget.MinTotalWorkloads != 5 || r.Spec.NamespaceBudget.FallbackUnhealthyCount != 2 {
		t.Fatalf("namespace budget defaults not applied")
	}
	if r.Spec.EmergencyTry.MaxAttempts != 1 {
		t.Fatalf("emergency try defaults not applied")
	}
}

func TestValidateAllowsStatefulSet(t *testing.T) {
	r := baseRequest()
	r.Spec.Workload.Kind = "StatefulSet"
	r.ApplyDefaults()
	if err := r.Validate(); err != nil {
		t.Fatalf("expected statefulset to be allowed, got err: %v", err)
	}
}

func TestValidateRejectsUnsupportedKind(t *testing.T) {
	r := baseRequest()
	r.Spec.Workload.Kind = "Job"
	r.ApplyDefaults()
	if err := r.Validate(); err == nil {
		t.Fatalf("expected validation error for unsupported workload kind")
	}
}

func TestValidateBoundaries(t *testing.T) {
	r := baseRequest()
	r.ApplyDefaults()
	r.Spec.BlastRadius.MaxPodPercentage = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected boundary validation error")
	}
	r.Spec.BlastRadius.MaxPodPercentage = 10
	r.Spec.IdempotencyWindowMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected idempotency window validation error")
	}
	r.Spec.IdempotencyWindowMinutes = 5
	r.Spec.CircuitBreaker.CooldownMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected cooldown validation error")
	}
	r.Spec.CircuitBreaker.CooldownMinutes = 10
	r.Spec.HealthyRevision.ObserveMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected healthy revision observe window validation error")
	}
	r.Spec.HealthyRevision.ObserveMinutes = 5
	r.Spec.NamespaceBudget.BlockingThresholdPercent = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected namespace budget threshold validation error")
	}
}
