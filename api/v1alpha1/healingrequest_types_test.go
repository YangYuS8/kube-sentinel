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
	if !r.Spec.SnapshotPolicy.Enabled {
		t.Fatalf("snapshot policy default enabled not applied")
	}
	if r.Spec.SnapshotPolicy.RetentionMinutes != 60 {
		t.Fatalf("snapshot retention default not applied")
	}
	if r.Spec.SnapshotPolicy.RestoreTimeoutSeconds != 30 {
		t.Fatalf("snapshot restore timeout default not applied")
	}
	if r.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload != 20 {
		t.Fatalf("snapshot max snapshots default not applied")
	}
	if r.Spec.DeploymentPolicy.L2CandidateWindowMinutes != 30 {
		t.Fatalf("deployment l2 candidate window default not applied")
	}
	if r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent != 10 {
		t.Fatalf("deployment l2 max degrade default not applied")
	}
	if r.Spec.DeploymentPolicy.L1SuccessRateMinPercent != 60 || r.Spec.DeploymentPolicy.L2SuccessRateMinPercent != 50 {
		t.Fatalf("deployment success rate defaults not applied")
	}
	if r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent != 40 || r.Spec.DeploymentPolicy.BlockRateMaxPercent != 30 {
		t.Fatalf("deployment gate rate defaults not applied")
	}
}

func TestValidateAllowsStatefulSet(t *testing.T) {
	r := baseRequest()
	r.Spec.Workload.Kind = "StatefulSet"
	r.ApplyDefaults()
	if err := r.Validate(); err != nil {
		t.Fatalf("expected statefulset to be allowed, got err: %v", err)
	}
	if r.Spec.StatefulSetPolicy.ApprovalAnnotation == "" {
		t.Fatalf("expected default approval annotation")
	}
	if r.Spec.StatefulSetPolicy.FreezeWindowMinutes < 1 {
		t.Fatalf("expected freeze window default")
	}
	if r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes < 1 {
		t.Fatalf("expected l2 candidate window default")
	}
	if r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent < 1 {
		t.Fatalf("expected l2 max degrade rate default")
	}
	if len(r.Spec.StatefulSetPolicy.AllowedNamespaces) != 1 || r.Spec.StatefulSetPolicy.AllowedNamespaces[0] != "default" {
		t.Fatalf("expected allowed namespace default to workload namespace")
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

func TestValidateStatefulSetPolicyBoundaries(t *testing.T) {
	r := baseRequest()
	r.Spec.Workload.Kind = "StatefulSet"
	r.ApplyDefaults()
	r.Spec.StatefulSetPolicy.FreezeWindowMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected freeze window validation error")
	}
	r.Spec.StatefulSetPolicy.FreezeWindowMinutes = 10
	r.Spec.StatefulSetPolicy.AllowedNamespaces = nil
	if err := r.Validate(); err == nil {
		t.Fatalf("expected allowed namespaces validation error")
	}
	r.Spec.StatefulSetPolicy.AllowedNamespaces = []string{"default"}
	r.Spec.StatefulSetPolicy.ApprovalAnnotation = ""
	if err := r.Validate(); err == nil {
		t.Fatalf("expected approval annotation validation error")
	}
	r.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected l2 candidate window validation error")
	}
	r.Spec.StatefulSetPolicy.L2CandidateWindowMinutes = 30
	r.Spec.StatefulSetPolicy.L2MaxDegradeRatePercent = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected l2 max degrade rate validation error")
	}
}

func TestValidateSnapshotPolicyBoundaries(t *testing.T) {
	r := baseRequest()
	r.ApplyDefaults()
	r.Spec.SnapshotPolicy.RetentionMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected snapshot retention validation error")
	}
	r.Spec.SnapshotPolicy.RetentionMinutes = 60
	r.Spec.SnapshotPolicy.RestoreTimeoutSeconds = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected snapshot restore timeout validation error")
	}
	r.Spec.SnapshotPolicy.RestoreTimeoutSeconds = 30
	r.Spec.SnapshotPolicy.MaxSnapshotsPerWorkload = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected snapshot max count validation error")
	}
}

func TestValidateDeploymentPolicyBoundaries(t *testing.T) {
	r := baseRequest()
	r.ApplyDefaults()
	r.Spec.DeploymentPolicy.L2CandidateWindowMinutes = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment l2 candidate window validation error")
	}
	r.Spec.DeploymentPolicy.L2CandidateWindowMinutes = 30
	r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment l2 max degrade validation error")
	}
	r.Spec.DeploymentPolicy.L2MaxDegradeRatePercent = 10
	r.Spec.DeploymentPolicy.L1SuccessRateMinPercent = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment l1 success rate validation error")
	}
	r.Spec.DeploymentPolicy.L1SuccessRateMinPercent = 60
	r.Spec.DeploymentPolicy.L2SuccessRateMinPercent = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment l2 success rate validation error")
	}
	r.Spec.DeploymentPolicy.L2SuccessRateMinPercent = 50
	r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent = 0
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment l3 degrade rate validation error")
	}
	r.Spec.DeploymentPolicy.L3DegradeRateMaxPercent = 40
	r.Spec.DeploymentPolicy.BlockRateMaxPercent = 101
	if err := r.Validate(); err == nil {
		t.Fatalf("expected deployment block rate validation error")
	}
}
