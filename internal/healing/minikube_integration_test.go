package healing

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

func TestMinikubeDeploymentL2Reality(t *testing.T) {
	if os.Getenv("KUBE_SENTINEL_MINIKUBE_INTEGRATION") != "true" {
		t.Skip("set KUBE_SENTINEL_MINIKUBE_INTEGRATION=true to run real minikube integration checks")
	}
	ctx := context.Background()
	cfg, err := config.GetConfig()
	if err != nil {
		t.Fatalf("load kube config: %v", err)
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("create k8s client: %v", err)
	}

	namespace := fmt.Sprintf("kube-sentinel-l2-reality-%d", time.Now().UnixNano())
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if err := k8sClient.Create(ctx, ns); err != nil {
		t.Fatalf("create namespace: %v", err)
	}
	defer func() {
		_ = k8sClient.Delete(context.Background(), ns)
	}()

	cmV1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg-v1", Namespace: namespace}, Data: map[string]string{"version": "v1"}}
	cmV2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg-v2", Namespace: namespace}, Data: map[string]string{"version": "v2"}}
	if err := k8sClient.Create(ctx, cmV1); err != nil {
		t.Fatalf("create cm v1: %v", err)
	}
	if err := k8sClient.Create(ctx, cmV2); err != nil {
		t.Fatalf("create cm v2: %v", err)
	}

	replicas := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: namespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "demo"}, Annotations: map[string]string{"kube-sentinel.io/test-rev": "1"}},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx:1.27-alpine",
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v1"}},
					}},
				}}},
			},
		},
	}
	if err := k8sClient.Create(ctx, dep); err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	waitForDeploymentReady(t, ctx, k8sClient, namespace, dep.Name)

	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dep.Name}, dep); err != nil {
		t.Fatalf("get deployment for rev2 update: %v", err)
	}
	dep.Spec.Template.Annotations["kube-sentinel.io/test-rev"] = "2"
	dep.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v2"}}}}
	if err := k8sClient.Update(ctx, dep); err != nil {
		t.Fatalf("update deployment to revision 2: %v", err)
	}
	waitForDeploymentReady(t, ctx, k8sClient, namespace, dep.Name)

	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dep.Name}, dep); err != nil {
		t.Fatalf("get deployment after rev2 rollout: %v", err)
	}
	currentRevision := dep.Annotations[deploymentRevisionAnnotation]
	adapter := NewDeploymentAdapter(k8sClient)
	records, err := adapter.ListRevisions(ctx, namespace, dep.Name)
	if err != nil {
		t.Fatalf("list revisions: %v", err)
	}
	t.Logf("current deployment revision=%s records=%+v", currentRevision, records)
	if len(records) < 2 {
		t.Fatalf("expected at least two revision records, got %d (%+v)", len(records), records)
	}

	historicalHealthy := ""
	for _, record := range records {
		if record.Revision != currentRevision && record.Healthy {
			historicalHealthy = record.Revision
			break
		}
	}
	if historicalHealthy == "" {
		t.Fatalf("no historical healthy revision observable on real minikube deployment; current=%s records=%+v", currentRevision, records)
	}

	if err := adapter.RollbackToRevision(ctx, namespace, dep.Name, historicalHealthy); err != nil {
		t.Fatalf("rollback to historical revision %s failed: %v", historicalHealthy, err)
	}
	waitForDeploymentReady(t, ctx, k8sClient, namespace, dep.Name)
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: dep.Name}, dep); err != nil {
		t.Fatalf("get deployment after rollback: %v", err)
	}
	if len(dep.Spec.Template.Spec.Containers) == 0 || len(dep.Spec.Template.Spec.Containers[0].EnvFrom) == 0 || dep.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef == nil {
		t.Fatalf("deployment envFrom lost after rollback")
	}
	if got := dep.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef.Name; got != "cfg-v1" {
		t.Fatalf("expected rollback to cfg-v1, got %s", got)
	}
}

func waitForDeploymentReady(t *testing.T, ctx context.Context, c client.Client, namespace, name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		dep := &appsv1.Deployment{}
		err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, dep)
		if apierrors.IsNotFound(err) {
			time.Sleep(2 * time.Second)
			continue
		}
		if err != nil {
			t.Fatalf("get deployment %s/%s: %v", namespace, name, err)
		}
		if dep.Generation == dep.Status.ObservedGeneration && dep.Status.AvailableReplicas >= 1 && dep.Status.ReadyReplicas >= 1 && dep.Status.UpdatedReplicas >= 1 {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timeout waiting for deployment %s/%s ready", namespace, name)
}
