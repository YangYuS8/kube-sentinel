package healing

import (
	"context"
	"errors"
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
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/yangyus8/kube-sentinel/internal/observability"
)

type statefulSetRealityFailingAdapter struct {
	DeploymentAdapter
}

func (a statefulSetRealityFailingAdapter) ExecuteStatefulSetControlledAction(context.Context, string, string, string) error {
	return errors.New("injected statefulset l1 failure for reality test")
}

func (a statefulSetRealityFailingAdapter) RollbackToRevision(context.Context, string, string, string) error {
	return errors.New("injected statefulset l2 rollback failure for reality test")
}

func TestMinikubeDeploymentL2Reality(t *testing.T) {
	if os.Getenv("KUBE_SENTINEL_MINIKUBE_INTEGRATION") != "true" {
		t.Skip("set KUBE_SENTINEL_MINIKUBE_INTEGRATION=true to run real minikube integration checks")
	}
	ctx := context.Background()
	k8sClient := newMinikubeClient(t)

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

func TestMinikubeStatefulSetRollbackReality(t *testing.T) {
	skipUnlessMinikubeStatefulSetRealityEnabled(t)
	ctx := context.Background()
	k8sClient := newMinikubeClient(t)
	adapter := NewDeploymentAdapter(k8sClient)

	namespace := fmt.Sprintf("kube-sentinel-sts-reality-%d", time.Now().UnixNano())
	createNamespace(t, ctx, k8sClient, namespace)
	defer func() {
		_ = k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	}()

	createStatefulSetRealityFixture(t, ctx, k8sClient, namespace, "demo", true)
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")
	rolloutStatefulSetToRevisionTwo(t, ctx, k8sClient, namespace, "demo")
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")

	statefulSet := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "demo"}, statefulSet); err != nil {
		t.Fatalf("get statefulset after rev2 rollout: %v", err)
	}
	records, err := adapter.ListRevisions(ctx, namespace, "demo")
	if err != nil {
		t.Fatalf("list statefulset revisions: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected at least two statefulset revision records, got %d (%+v)", len(records), records)
	}
	historicalHealthy := ""
	for _, record := range records {
		if record.Revision == statefulSet.Status.CurrentRevision || record.Revision == statefulSet.Status.UpdateRevision {
			if record.Healthy {
				t.Fatalf("current statefulset revision should not remain a healthy rollback candidate on real cluster: %+v", records)
			}
			continue
		}
		if record.Healthy {
			historicalHealthy = record.Revision
			break
		}
	}
	if historicalHealthy == "" {
		t.Fatalf("no historical healthy statefulset revision observable on real minikube cluster; current=%s update=%s records=%+v", statefulSet.Status.CurrentRevision, statefulSet.Status.UpdateRevision, records)
	}
	if err := adapter.ValidateRevisionDependencies(ctx, namespace, "demo", historicalHealthy); err != nil {
		t.Fatalf("validate statefulset dependencies for %s: %v", historicalHealthy, err)
	}
	if err := adapter.RollbackToRevision(ctx, namespace, "demo", historicalHealthy); err != nil {
		t.Fatalf("rollback to historical statefulset revision %s failed: %v", historicalHealthy, err)
	}
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "demo"}, statefulSet); err != nil {
		t.Fatalf("get statefulset after rollback: %v", err)
	}
	if got := statefulSet.Spec.Template.Annotations["kube-sentinel.io/test-rev"]; got != "1" {
		t.Fatalf("expected rollback to restore template revision marker 1, got %s", got)
	}
	if len(statefulSet.Spec.Template.Spec.Containers) == 0 || len(statefulSet.Spec.Template.Spec.Containers[0].EnvFrom) == 0 || statefulSet.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef == nil {
		t.Fatalf("statefulset envFrom lost after rollback")
	}
	if got := statefulSet.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef.Name; got != "cfg-v1" {
		t.Fatalf("expected rollback to cfg-v1, got %s", got)
	}
	t.Logf("STATEFULSET_REALITY_CONTEXT=minikube")
	t.Logf("STATEFULSET_REALITY_SCENARIO=historical-rollback")
	t.Logf("STATEFULSET_REALITY_CANDIDATE=%s", historicalHealthy)
	t.Logf("STATEFULSET_REALITY_RESULT=pass")
}

func TestMinikubeStatefulSetNoHistoricalCandidateReality(t *testing.T) {
	skipUnlessMinikubeStatefulSetRealityEnabled(t)
	ctx := context.Background()
	k8sClient := newMinikubeClient(t)
	adapter := NewDeploymentAdapter(k8sClient)

	namespace := fmt.Sprintf("kube-sentinel-sts-no-candidate-%d", time.Now().UnixNano())
	createNamespace(t, ctx, k8sClient, namespace)
	defer func() {
		_ = k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	}()

	createStatefulSetRealityFixture(t, ctx, k8sClient, namespace, "demo", false)
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")
	records, err := adapter.ListRevisions(ctx, namespace, "demo")
	if err != nil {
		t.Fatalf("list statefulset revisions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly one current statefulset revision, got %+v", records)
	}
	if records[0].Healthy {
		t.Fatalf("expected lone current statefulset revision to be excluded from healthy rollback candidates, got %+v", records)
	}
	if _, err := SelectLatestHealthyRevision(records); err == nil {
		t.Fatalf("expected no healthy historical statefulset candidate, got %+v", records)
	}
	t.Logf("STATEFULSET_REALITY_CONTEXT=minikube")
	t.Logf("STATEFULSET_REALITY_SCENARIO=no-historical-candidate")
	t.Logf("STATEFULSET_REALITY_RESULT=pass")
}

func TestMinikubeStatefulSetL2FreezeAndRestoreEvidence(t *testing.T) {
	skipUnlessMinikubeStatefulSetRealityEnabled(t)
	ctx := context.Background()
	k8sClient := newMinikubeClient(t)

	namespace := fmt.Sprintf("kube-sentinel-sts-freeze-%d", time.Now().UnixNano())
	createNamespace(t, ctx, k8sClient, namespace)
	defer func() {
		_ = k8sClient.Delete(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
	}()

	createStatefulSetRealityFixture(t, ctx, k8sClient, namespace, "demo", true)
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")
	rolloutStatefulSetToRevisionTwo(t, ctx, k8sClient, namespace, "demo")
	waitForStatefulSetReady(t, ctx, k8sClient, namespace, "demo")

	req := newReq()
	req.Spec.Workload.Kind = "StatefulSet"
	req.Spec.Workload.Namespace = namespace
	req.Spec.Workload.Name = "demo"
	req.Spec.StatefulSetPolicy.Enabled = true
	req.Spec.StatefulSetPolicy.ReadOnlyOnly = false
	req.Spec.StatefulSetPolicy.ControlledActionsEnabled = true
	req.Spec.StatefulSetPolicy.L2RollbackEnabled = true
	req.Spec.StatefulSetPolicy.RequireEvidence = false
	req.Spec.StatefulSetPolicy.FreezeWindowMinutes = 5
	req.Spec.StatefulSetPolicy.ApprovalAnnotation = "kube-sentinel.io/statefulset-approved"
	req.Spec.StatefulSetPolicy.AllowedNamespaces = []string{namespace}
	req.Annotations = map[string]string{"kube-sentinel.io/statefulset-approved": "true"}
	audits := &observability.MemoryAuditSink{}
	events := &observability.MemoryEventSink{}
	metrics := &observability.Metrics{}
	now := time.Unix(1710000000, 0)
	orchestrator := &Orchestrator{
		Adapter:              statefulSetRealityFailingAdapter{DeploymentAdapter: NewDeploymentAdapter(k8sClient)},
		Snapshotter:          NewKubernetesSnapshotter(k8sClient),
		RuntimeInputProvider: fakeRuntimeInputProvider{input: RuntimeInput{AffectedPods: 1, ClusterPods: 100, TotalWorkloads: 10, UnhealthyWorkloads: 1}},
		AuditSink:            audits,
		EventSink:            events,
		Metrics:              metrics,
		Now:                  func() time.Time { return now },
	}
	if _, err := orchestrator.Process(ctx, req); err == nil {
		t.Fatalf("expected injected statefulset rollback failure to bubble up")
	}
	if req.Status.StatefulSetFreezeState != "frozen" || req.Status.StatefulSetFreezeUntil == "" {
		t.Fatalf("expected real-cluster statefulset l2 failure to freeze workload, got %s/%s", req.Status.StatefulSetFreezeState, req.Status.StatefulSetFreezeUntil)
	}
	if req.Status.SnapshotRestoreResult != "success" {
		t.Fatalf("expected real-cluster snapshot restore success evidence, got %s", req.Status.SnapshotRestoreResult)
	}
	if req.Status.LastSnapshotID == "" {
		t.Fatalf("expected last snapshot id to be recorded")
	}
	if len(events.Events) == 0 || events.Events[len(events.Events)-1].Reason != "StatefulSetL2RollbackFailed" {
		t.Fatalf("expected rollback failure event evidence, got %+v", events.Events)
	}
	if len(audits.Events) == 0 || audits.Events[len(audits.Events)-1].FreezeState != "frozen" {
		t.Fatalf("expected frozen audit evidence, got %+v", audits.Events)
	}
	statefulSet := &appsv1.StatefulSet{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: "demo"}, statefulSet); err != nil {
		t.Fatalf("get statefulset after rollback failure restore: %v", err)
	}
	if got := statefulSet.Spec.Template.Annotations["kube-sentinel.io/test-rev"]; got != "2" {
		t.Fatalf("expected snapshot restore to keep revision marker 2, got %s", got)
	}
	if len(statefulSet.Spec.Template.Spec.Containers) == 0 || len(statefulSet.Spec.Template.Spec.Containers[0].EnvFrom) == 0 || statefulSet.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef == nil {
		t.Fatalf("statefulset envFrom lost after snapshot restore")
	}
	if got := statefulSet.Spec.Template.Spec.Containers[0].EnvFrom[0].ConfigMapRef.Name; got != "cfg-v2" {
		t.Fatalf("expected snapshot restore to keep cfg-v2, got %s", got)
	}
	t.Logf("STATEFULSET_REALITY_CONTEXT=minikube")
	t.Logf("STATEFULSET_REALITY_SCENARIO=l2-freeze-and-restore")
	t.Logf("STATEFULSET_REALITY_RESULT=pass")
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

func skipUnlessMinikubeStatefulSetRealityEnabled(t *testing.T) {
	t.Helper()
	if os.Getenv("KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY") != "true" {
		t.Skip("set KUBE_SENTINEL_MINIKUBE_STATEFULSET_REALITY=true to run real StatefulSet minikube checks")
	}
}

func newMinikubeClient(t *testing.T) client.Client {
	t.Helper()
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
	return k8sClient
}

func createNamespace(t *testing.T, ctx context.Context, c client.Client, namespace string) {
	t.Helper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if err := c.Create(ctx, ns); err != nil {
		t.Fatalf("create namespace %s: %v", namespace, err)
	}
}

func createStatefulSetRealityFixture(t *testing.T, ctx context.Context, c client.Client, namespace, name string, createSecondConfig bool) {
	t.Helper()
	cmV1 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg-v1", Namespace: namespace}, Data: map[string]string{"version": "v1"}}
	if err := c.Create(ctx, cmV1); err != nil {
		t.Fatalf("create cm v1: %v", err)
	}
	if createSecondConfig {
		cmV2 := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "cfg-v2", Namespace: namespace}, Data: map[string]string{"version": "v2"}}
		if err := c.Create(ctx, cmV2); err != nil {
			t.Fatalf("create cm v2: %v", err)
		}
	}
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
			Selector:  map[string]string{"app": name},
			Ports:     []corev1.ServicePort{{Port: 80, TargetPort: intstrFromInt(80)}},
		},
	}
	if err := c.Create(ctx, service); err != nil {
		t.Fatalf("create headless service: %v", err)
	}
	replicas := int32(1)
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    &replicas,
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      map[string]string{"app": name},
					Annotations: map[string]string{"kube-sentinel.io/test-rev": "1"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name:  "nginx",
					Image: "nginx:1.27-alpine",
					Ports: []corev1.ContainerPort{{ContainerPort: 80}},
					EnvFrom: []corev1.EnvFromSource{{
						ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v1"}},
					}},
				}}},
			},
		},
	}
	if err := c.Create(ctx, statefulSet); err != nil {
		t.Fatalf("create statefulset fixture: %v", err)
	}
}

func rolloutStatefulSetToRevisionTwo(t *testing.T, ctx context.Context, c client.Client, namespace, name string) {
	t.Helper()
	statefulSet := &appsv1.StatefulSet{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, statefulSet); err != nil {
		t.Fatalf("get statefulset for revision 2 rollout: %v", err)
	}
	statefulSet.Spec.Template.Annotations["kube-sentinel.io/test-rev"] = "2"
	statefulSet.Spec.Template.Spec.Containers[0].EnvFrom = []corev1.EnvFromSource{{
		ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg-v2"}},
	}}
	if err := c.Update(ctx, statefulSet); err != nil {
		t.Fatalf("update statefulset to revision 2: %v", err)
	}
}

func waitForStatefulSetReady(t *testing.T, ctx context.Context, c client.Client, namespace, name string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		statefulSet := &appsv1.StatefulSet{}
		err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, statefulSet)
		if apierrors.IsNotFound(err) {
			time.Sleep(2 * time.Second)
			continue
		}
		if err != nil {
			t.Fatalf("get statefulset %s/%s: %v", namespace, name, err)
		}
		expectedReplicas := int32(1)
		if statefulSet.Spec.Replicas != nil {
			expectedReplicas = *statefulSet.Spec.Replicas
		}
		if statefulSet.Generation == statefulSet.Status.ObservedGeneration && statefulSet.Status.ReadyReplicas >= expectedReplicas && statefulSet.Status.UpdatedReplicas >= expectedReplicas && statefulSet.Status.CurrentRevision != "" && statefulSet.Status.CurrentRevision == statefulSet.Status.UpdateRevision {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timeout waiting for statefulset %s/%s ready", namespace, name)
}

func intstrFromInt(value int) intstr.IntOrString {
	return intstr.FromInt(value)
}
