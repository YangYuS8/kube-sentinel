package healing

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeploymentAdapterListRevisions(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "app"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "3",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(2)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 2, AvailableReplicas: 2},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs).Build()
	adapter := NewDeploymentAdapter(cl)
	revs, err := adapter.ListRevisions(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(revs) != 1 || revs[0].Revision != "3" || !revs[0].Healthy {
		t.Fatalf("unexpected revisions result: %+v", revs)
	}
}

func TestDeploymentAdapterListRevisionsMarksHistoricalScaledDownReplicaSetHealthy(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app",
			Namespace: "default",
			UID:       types.UID("dep-uid"),
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "app"}}},
		},
	}
	oldRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation:                "1",
				"deployment.kubernetes.io/desired-replicas": "1",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(0)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 0, AvailableReplicas: 0},
	}
	currentRS := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-2",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation:                "2",
				"deployment.kubernetes.io/desired-replicas": "1",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec:   appsv1.ReplicaSetSpec{Replicas: ptrInt32(1)},
		Status: appsv1.ReplicaSetStatus{ReadyReplicas: 1, AvailableReplicas: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, oldRS, currentRS).Build()
	adapter := NewDeploymentAdapter(cl)
	revs, err := adapter.ListRevisions(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(revs) != 2 {
		t.Fatalf("expected two revisions, got %+v", revs)
	}
	if !revs[0].Healthy && !revs[1].Healthy {
		t.Fatalf("expected at least one healthy revision, got %+v", revs)
	}
	foundHistoricalHealthy := false
	for _, revision := range revs {
		if revision.Revision == "1" && revision.Healthy {
			foundHistoricalHealthy = true
		}
	}
	if !foundHistoricalHealthy {
		t.Fatalf("expected scaled-down historical revision to remain rollback-eligible, got %+v", revs)
	}
}

func TestDeploymentAdapterRollbackToRevision(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "new"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "stable"}}}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs).Build()
	adapter := NewDeploymentAdapter(cl)
	if err := adapter.RollbackToRevision(context.Background(), "default", "app", "2"); err != nil {
		t.Fatalf("unexpected rollback err: %v", err)
	}
	updated := &appsv1.Deployment{}
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "app"}, updated); err != nil {
		t.Fatalf("get updated deployment failed: %v", err)
	}
	if updated.Spec.Template.Labels["version"] != "stable" {
		t.Fatalf("expected deployment template rollback to stable, got %s", updated.Spec.Template.Labels["version"])
	}
}

func TestDeploymentAdapterCountPods(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	replicas := int32(3)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}}},
		Status:     appsv1.DeploymentStatus{Replicas: 3},
	}
	pod1 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p1", Namespace: "default"}}
	pod2 := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p2", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, pod1, pod2).Build()
	adapter := NewDeploymentAdapter(cl)
	affected, err := adapter.CountAffectedPods(context.Background(), "default", "app")
	if err != nil {
		t.Fatalf("count affected failed: %v", err)
	}
	if affected != 3 {
		t.Fatalf("expected affected pods 3, got %d", affected)
	}
	cluster, err := adapter.CountClusterPods(context.Background(), "default")
	if err != nil {
		t.Fatalf("count cluster pods failed: %v", err)
	}
	if cluster != 2 {
		t.Fatalf("expected cluster pod count 2, got %d", cluster)
	}
}

func TestDeploymentAdapterValidateRevisionDependencies(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default", UID: types.UID("dep-uid")},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"version": "new"}}},
		},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-rs-1",
			Namespace: "default",
			Annotations: map[string]string{
				deploymentRevisionAnnotation: "2",
			},
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", UID: dep.UID, Controller: ptrBool(true)}},
		},
		Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Volumes:    []corev1.Volume{{Name: "cfg", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"}}}}},
				Containers: []corev1.Container{{Name: "app", EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "sec"}}}}}},
			},
		}},
	}
	cfg := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"}}
	sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "sec", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep, rs, cfg, sec).Build()
	adapter := NewDeploymentAdapter(cl)
	if err := adapter.ValidateRevisionDependencies(context.Background(), "default", "app", "2"); err != nil {
		t.Fatalf("expected dependencies valid, got %v", err)
	}
}

func TestDeploymentAdapterCountWorkloads(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	dep1Replicas := int32(3)
	dep2Replicas := int32(2)
	dep1 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app1", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &dep1Replicas}, Status: appsv1.DeploymentStatus{AvailableReplicas: 1}}
	dep2 := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "app2", Namespace: "default"}, Spec: appsv1.DeploymentSpec{Replicas: &dep2Replicas}, Status: appsv1.DeploymentStatus{AvailableReplicas: 2}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep1, dep2).Build()
	adapter := NewDeploymentAdapter(cl)
	total, err := adapter.CountTotalWorkloads(context.Background(), "default")
	if err != nil || total != 2 {
		t.Fatalf("expected total workloads 2, got %d err=%v", total, err)
	}
	unhealthy, err := adapter.CountUnhealthyWorkloads(context.Background(), "default")
	if err != nil || unhealthy != 1 {
		t.Fatalf("expected unhealthy workloads 1, got %d err=%v", unhealthy, err)
	}
}

func ptrBool(v bool) *bool    { return &v }
func ptrInt32(v int32) *int32 { return &v }
