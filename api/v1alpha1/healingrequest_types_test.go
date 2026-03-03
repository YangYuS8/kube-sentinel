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
}

func TestValidateRejectsNonDeployment(t *testing.T) {
	r := baseRequest()
	r.Spec.Workload.Kind = "StatefulSet"
	r.ApplyDefaults()
	if err := r.Validate(); err == nil {
		t.Fatalf("expected validation error for non-deployment")
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
}
