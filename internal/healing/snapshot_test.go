package healing

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func newSnapshotterTestClient(t *testing.T, objects ...runtime.Object) *KubernetesSnapshotter {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &KubernetesSnapshotter{Client: cli, Now: func() time.Time { return time.Unix(1000, 0).UTC() }}
}

func TestKubernetesSnapshotterCreateAndRestoreDeployment(t *testing.T) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "app", Annotations: map[string]string{deploymentRevisionAnnotation: "5"}},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{"k": "v1"}},
			},
		},
	}
	snapshotter := newSnapshotterTestClient(t, deployment)
	snapshot, err := snapshotter.Create(ctx, "default", "app", SnapshotOptions{
		WorkloadKind:      "Deployment",
		Phase:             "deployment-l1",
		IdempotencyKey:    "default/app/deployment-l1/1",
		RetentionMinutes:  60,
		MaxSnapshotsCount: 10,
	})
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if snapshot.ID == "" {
		t.Fatalf("expected snapshot id")
	}

	current := appsv1.Deployment{}
	if err := snapshotter.Client.Get(ctx, types.NamespacedName{Namespace: "default", Name: "app"}, &current); err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	current.Spec.Template.Annotations["k"] = "mutated"
	if err := snapshotter.Client.Update(ctx, &current); err != nil {
		t.Fatalf("mutate deployment: %v", err)
	}

	if err := snapshotter.Restore(ctx, snapshot); err != nil {
		t.Fatalf("restore snapshot: %v", err)
	}
	restored := appsv1.Deployment{}
	if err := snapshotter.Client.Get(ctx, types.NamespacedName{Namespace: "default", Name: "app"}, &restored); err != nil {
		t.Fatalf("get restored deployment: %v", err)
	}
	if restored.Spec.Template.Annotations["k"] != "v1" {
		t.Fatalf("expected restored template annotation, got %q", restored.Spec.Template.Annotations["k"])
	}
}

func TestKubernetesSnapshotterIdempotencyAndCapacity(t *testing.T) {
	ctx := context.Background()
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "db"},
		Spec: appsv1.StatefulSetSpec{
			Template: corev1.PodTemplateSpec{},
		},
		Status: appsv1.StatefulSetStatus{CurrentRevision: "rev-1"},
	}
	snapshotter := newSnapshotterTestClient(t, statefulSet)

	first, err := snapshotter.Create(ctx, "default", "db", SnapshotOptions{
		WorkloadKind:      "StatefulSet",
		Phase:             "statefulset-l1",
		IdempotencyKey:    "default/db/statefulset-l1/1",
		RetentionMinutes:  60,
		MaxSnapshotsCount: 1,
	})
	if err != nil {
		t.Fatalf("create first snapshot: %v", err)
	}
	again, err := snapshotter.Create(ctx, "default", "db", SnapshotOptions{
		WorkloadKind:      "StatefulSet",
		Phase:             "statefulset-l1",
		IdempotencyKey:    "default/db/statefulset-l1/1",
		RetentionMinutes:  60,
		MaxSnapshotsCount: 1,
	})
	if err != nil {
		t.Fatalf("create duplicate snapshot: %v", err)
	}
	if first.ID != again.ID {
		t.Fatalf("expected idempotent snapshot reuse")
	}
	if _, err := snapshotter.Create(ctx, "default", "db", SnapshotOptions{
		WorkloadKind:      "StatefulSet",
		Phase:             "statefulset-l2",
		IdempotencyKey:    "default/db/statefulset-l2/2",
		RetentionMinutes:  60,
		MaxSnapshotsCount: 1,
	}); err == nil {
		t.Fatalf("expected capacity block")
	}
}

func TestBuildRecoveryGateImpact(t *testing.T) {
	allow := BuildRecoveryGateImpact(nil, nil)
	if allow.GateEffect != "allow" || allow.RequiresManualIntervention {
		t.Fatalf("expected allow when rollback not failed")
	}

	restored := BuildRecoveryGateImpact(errors.New("rollback failed"), nil)
	if restored.GateEffect != "block" || restored.ReasonCode != "rollback_failed_snapshot_restored" {
		t.Fatalf("expected block with snapshot restored reason")
	}

	failed := BuildRecoveryGateImpact(errors.New("rollback failed"), errors.New("restore failed"))
	if failed.GateEffect != "block" || failed.ReasonCode != "rollback_failed_restore_failed" {
		t.Fatalf("expected block with restore failure reason")
	}
}
