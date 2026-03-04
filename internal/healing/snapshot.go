package healing

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	snapshotManagedByLabel = "kube-sentinel.io/managed-by"
	snapshotMarkerLabel    = "kube-sentinel.io/snapshot"
	snapshotNamespaceLabel = "kube-sentinel.io/workload-namespace"
	snapshotNameLabel      = "kube-sentinel.io/workload-name"
	snapshotKindLabel      = "kube-sentinel.io/workload-kind"

	snapshotIDAnnotation            = "kube-sentinel.io/snapshot-id"
	snapshotIdempotencyAnnotation   = "kube-sentinel.io/snapshot-idempotency-key"
	snapshotRestoreResultAnnotation = "kube-sentinel.io/snapshot-restore-result"
)

type Snapshot struct {
	ID             string
	ResourceName   string
	Namespace      string
	Name           string
	WorkloadKind   string
	Revision       string
	Phase          string
	IdempotencyKey string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

type SnapshotOptions struct {
	WorkloadKind      string
	Phase             string
	IdempotencyKey    string
	RetentionMinutes  int
	MaxSnapshotsCount int
}

type RecoveryGateImpact struct {
	RecoveryResult             string
	GateEffect                 string
	ReasonCode                 string
	Recommendation             string
	RequiresManualIntervention bool
}

func BuildRecoveryGateImpact(rollbackErr error, restoreErr error) RecoveryGateImpact {
	if rollbackErr == nil {
		return RecoveryGateImpact{
			RecoveryResult:             "not-required",
			GateEffect:                 "allow",
			ReasonCode:                 "none",
			Recommendation:             "continue observing rollout stability",
			RequiresManualIntervention: false,
		}
	}
	if restoreErr == nil {
		return RecoveryGateImpact{
			RecoveryResult:             "snapshot-restored",
			GateEffect:                 "block",
			ReasonCode:                 "rollback_failed_snapshot_restored",
			Recommendation:             "hold write actions, inspect rollback cause, and request manual approval before retry",
			RequiresManualIntervention: true,
		}
	}
	return RecoveryGateImpact{
		RecoveryResult:             "snapshot-restore-failed",
		GateEffect:                 "block",
		ReasonCode:                 "rollback_failed_restore_failed",
		Recommendation:             "escalate immediately and keep conservative read-only mode until snapshot recovery is fixed",
		RequiresManualIntervention: true,
	}
}

type Snapshotter interface {
	Create(ctx context.Context, namespace, name string, options SnapshotOptions) (Snapshot, error)
	List(ctx context.Context, namespace, name string) ([]Snapshot, error)
	Restore(ctx context.Context, snapshot Snapshot) error
	Prune(ctx context.Context, namespace, name string, retentionMinutes, maxCount int) (int, error)
}

type MemorySnapshotter struct {
	mu        sync.Mutex
	Snapshots []Snapshot
}

func (m *MemorySnapshotter) Create(_ context.Context, namespace, name string, options SnapshotOptions) (Snapshot, error) {
	now := time.Now().UTC()
	id := shortHash(namespace + "/" + name + "/" + options.Phase + "/" + options.IdempotencyKey)
	for _, existing := range m.Snapshots {
		if existing.Namespace == namespace && existing.Name == name && existing.IdempotencyKey == options.IdempotencyKey && options.IdempotencyKey != "" {
			return existing, nil
		}
	}
	retention := durationFromMinutes(options.RetentionMinutes, 60)
	s := Snapshot{
		ID:             id,
		ResourceName:   "memory-" + id,
		Namespace:      namespace,
		Name:           name,
		WorkloadKind:   options.WorkloadKind,
		Revision:       "current",
		Phase:          options.Phase,
		IdempotencyKey: options.IdempotencyKey,
		CreatedAt:      now,
		ExpiresAt:      now.Add(retention),
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Snapshots = append(m.Snapshots, s)
	return s, nil
}

func (m *MemorySnapshotter) List(_ context.Context, namespace, name string) ([]Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Snapshot, 0, len(m.Snapshots))
	for _, s := range m.Snapshots {
		if s.Namespace == namespace && s.Name == name {
			out = append(out, s)
		}
	}
	return out, nil
}

func (m *MemorySnapshotter) Restore(_ context.Context, snapshot Snapshot) error {
	if snapshot.Name == "" || snapshot.Namespace == "" {
		return fmt.Errorf("invalid snapshot")
	}
	return nil
}

func (m *MemorySnapshotter) Prune(_ context.Context, namespace, name string, retentionMinutes, maxCount int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	retention := durationFromMinutes(retentionMinutes, 60)
	kept := make([]Snapshot, 0, len(m.Snapshots))
	workload := make([]Snapshot, 0)
	pruned := 0
	for _, s := range m.Snapshots {
		if s.Namespace == namespace && s.Name == name {
			if now.Sub(s.CreatedAt) > retention || (!s.ExpiresAt.IsZero() && now.After(s.ExpiresAt)) {
				pruned++
				continue
			}
			workload = append(workload, s)
			continue
		}
		kept = append(kept, s)
	}
	sort.Slice(workload, func(i, j int) bool {
		return workload[i].CreatedAt.After(workload[j].CreatedAt)
	})
	if maxCount > 0 && len(workload) > maxCount {
		pruned += len(workload) - maxCount
		workload = workload[:maxCount]
	}
	m.Snapshots = append(kept, workload...)
	return pruned, nil
}

type KubernetesSnapshotter struct {
	Client client.Client
	Now    func() time.Time
}

func NewKubernetesSnapshotter(k8sClient client.Client) *KubernetesSnapshotter {
	return &KubernetesSnapshotter{Client: k8sClient}
}

type workloadSnapshotPayload struct {
	Kind                   string                            `json:"kind"`
	Revision               string                            `json:"revision"`
	DeploymentTemplate     *corev1.PodTemplateSpec           `json:"deploymentTemplate,omitempty"`
	DeploymentAnnotations  map[string]string                 `json:"deploymentAnnotations,omitempty"`
	StatefulSetTemplate    *corev1.PodTemplateSpec           `json:"statefulSetTemplate,omitempty"`
	StatefulSetUpdate      *appsv1.StatefulSetUpdateStrategy `json:"statefulSetUpdate,omitempty"`
	StatefulSetAnnotations map[string]string                 `json:"statefulSetAnnotations,omitempty"`
}

func (k *KubernetesSnapshotter) Create(ctx context.Context, namespace, name string, options SnapshotOptions) (Snapshot, error) {
	if k.Client == nil {
		return Snapshot{}, fmt.Errorf("kubernetes client is required")
	}
	if namespace == "" || name == "" {
		return Snapshot{}, fmt.Errorf("snapshot target namespace/name is required")
	}
	now := k.now()
	if options.IdempotencyKey != "" {
		existing, err := k.findByIdempotencyKey(ctx, namespace, name, options.IdempotencyKey)
		if err == nil && existing.ID != "" {
			return existing, nil
		}
	}
	if _, err := k.Prune(ctx, namespace, name, options.RetentionMinutes, options.MaxSnapshotsCount); err != nil {
		return Snapshot{}, err
	}
	active, err := k.List(ctx, namespace, name)
	if err != nil {
		return Snapshot{}, err
	}
	if options.MaxSnapshotsCount > 0 && len(active) >= options.MaxSnapshotsCount {
		return Snapshot{}, fmt.Errorf("snapshot capacity exceeded for %s/%s", namespace, name)
	}
	payload, revision, err := k.capturePayload(ctx, namespace, name, options.WorkloadKind)
	if err != nil {
		return Snapshot{}, err
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return Snapshot{}, err
	}
	id := shortHash(strings.Join([]string{namespace, name, options.WorkloadKind, options.Phase, options.IdempotencyKey, strconv.FormatInt(now.UnixNano(), 10)}, "/"))
	cmName := "kube-sentinel-snapshot-" + id
	retention := durationFromMinutes(options.RetentionMinutes, 60)
	expiresAt := now.Add(retention)
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      cmName,
			Labels: map[string]string{
				snapshotManagedByLabel: "kube-sentinel",
				snapshotMarkerLabel:    "true",
				snapshotNamespaceLabel: safeLabelValue(namespace),
				snapshotNameLabel:      safeLabelValue(name),
				snapshotKindLabel:      safeLabelValue(strings.ToLower(options.WorkloadKind)),
			},
			Annotations: map[string]string{
				snapshotIDAnnotation:          id,
				snapshotIdempotencyAnnotation: options.IdempotencyKey,
			},
		},
		Data: map[string]string{
			"snapshotID":      id,
			"namespace":       namespace,
			"name":            name,
			"workloadKind":    options.WorkloadKind,
			"phase":           options.Phase,
			"revision":        revision,
			"idempotencyKey":  options.IdempotencyKey,
			"createdAt":       now.Format(time.RFC3339),
			"expiresAt":       expiresAt.Format(time.RFC3339),
			"snapshotPayload": string(payloadBytes),
		},
	}
	if err := k.Client.Create(ctx, &cm); err != nil {
		return Snapshot{}, err
	}
	return snapshotFromConfigMap(cm)
}

func (k *KubernetesSnapshotter) List(ctx context.Context, namespace, name string) ([]Snapshot, error) {
	if k.Client == nil {
		return nil, fmt.Errorf("kubernetes client is required")
	}
	configMaps := corev1.ConfigMapList{}
	if err := k.Client.List(ctx, &configMaps, client.InNamespace(namespace), client.MatchingLabels(map[string]string{
		snapshotManagedByLabel: "kube-sentinel",
		snapshotMarkerLabel:    "true",
		snapshotNameLabel:      safeLabelValue(name),
	})); err != nil {
		return nil, err
	}
	out := make([]Snapshot, 0, len(configMaps.Items))
	for _, cm := range configMaps.Items {
		s, err := snapshotFromConfigMap(cm)
		if err != nil {
			continue
		}
		if s.Namespace == namespace && s.Name == name {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (k *KubernetesSnapshotter) Restore(ctx context.Context, snapshot Snapshot) error {
	if k.Client == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	if snapshot.Namespace == "" || snapshot.ResourceName == "" {
		return fmt.Errorf("invalid snapshot")
	}
	cm := corev1.ConfigMap{}
	if err := k.Client.Get(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.ResourceName}, &cm); err != nil {
		return err
	}
	payloadRaw := cm.Data["snapshotPayload"]
	if payloadRaw == "" {
		return fmt.Errorf("snapshot payload is empty")
	}
	payload := workloadSnapshotPayload{}
	if err := json.Unmarshal([]byte(payloadRaw), &payload); err != nil {
		return err
	}
	switch payload.Kind {
	case "Deployment":
		deployment := appsv1.Deployment{}
		if err := k.Client.Get(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Name}, &deployment); err != nil {
			return err
		}
		if payload.DeploymentTemplate == nil {
			return fmt.Errorf("deployment snapshot template missing")
		}
		deployment.Spec.Template = *payload.DeploymentTemplate
		deployment.Annotations = cloneStringMap(payload.DeploymentAnnotations)
		if err := k.Client.Update(ctx, &deployment); err != nil {
			return err
		}
	case "StatefulSet":
		statefulSet := appsv1.StatefulSet{}
		if err := k.Client.Get(ctx, types.NamespacedName{Namespace: snapshot.Namespace, Name: snapshot.Name}, &statefulSet); err != nil {
			return err
		}
		if payload.StatefulSetTemplate == nil || payload.StatefulSetUpdate == nil {
			return fmt.Errorf("statefulset snapshot payload is incomplete")
		}
		statefulSet.Spec.Template = *payload.StatefulSetTemplate
		statefulSet.Spec.UpdateStrategy = *payload.StatefulSetUpdate
		statefulSet.Annotations = cloneStringMap(payload.StatefulSetAnnotations)
		if err := k.Client.Update(ctx, &statefulSet); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported snapshot kind %q", payload.Kind)
	}
	cm.Annotations = ensureStringMap(cm.Annotations)
	cm.Annotations[snapshotRestoreResultAnnotation] = "success"
	return k.Client.Update(ctx, &cm)
}

func (k *KubernetesSnapshotter) Prune(ctx context.Context, namespace, name string, retentionMinutes, maxCount int) (int, error) {
	if k.Client == nil {
		return 0, fmt.Errorf("kubernetes client is required")
	}
	snapshots, err := k.List(ctx, namespace, name)
	if err != nil {
		return 0, err
	}
	now := k.now()
	retention := durationFromMinutes(retentionMinutes, 60)
	pruned := 0
	for _, snapshot := range snapshots {
		if now.Sub(snapshot.CreatedAt) > retention || (!snapshot.ExpiresAt.IsZero() && now.After(snapshot.ExpiresAt)) {
			if err := k.Client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: snapshot.Namespace, Name: snapshot.ResourceName}}); err != nil && !apierrors.IsNotFound(err) {
				return pruned, err
			}
			pruned++
		}
	}
	if maxCount <= 0 {
		return pruned, nil
	}
	latest, err := k.List(ctx, namespace, name)
	if err != nil {
		return pruned, err
	}
	if len(latest) <= maxCount {
		return pruned, nil
	}
	for _, snapshot := range latest[maxCount:] {
		if err := k.Client.Delete(ctx, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: snapshot.Namespace, Name: snapshot.ResourceName}}); err != nil && !apierrors.IsNotFound(err) {
			return pruned, err
		}
		pruned++
	}
	return pruned, nil
}

func (k *KubernetesSnapshotter) findByIdempotencyKey(ctx context.Context, namespace, name, idempotencyKey string) (Snapshot, error) {
	snapshots, err := k.List(ctx, namespace, name)
	if err != nil {
		return Snapshot{}, err
	}
	for _, snapshot := range snapshots {
		if snapshot.IdempotencyKey == idempotencyKey {
			return snapshot, nil
		}
	}
	return Snapshot{}, fmt.Errorf("snapshot not found")
}

func (k *KubernetesSnapshotter) capturePayload(ctx context.Context, namespace, name, workloadKind string) (workloadSnapshotPayload, string, error) {
	kind := strings.TrimSpace(workloadKind)
	if kind == "" || strings.EqualFold(kind, "Deployment") {
		deployment := appsv1.Deployment{}
		if err := k.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &deployment); err == nil {
			payload := workloadSnapshotPayload{
				Kind:                  "Deployment",
				Revision:              deployment.Annotations[deploymentRevisionAnnotation],
				DeploymentTemplate:    deployment.Spec.Template.DeepCopy(),
				DeploymentAnnotations: cloneStringMap(deployment.Annotations),
			}
			if payload.Revision == "" {
				payload.Revision = "current"
			}
			return payload, payload.Revision, nil
		} else if !apierrors.IsNotFound(err) {
			return workloadSnapshotPayload{}, "", err
		}
	}
	if kind == "" || strings.EqualFold(kind, "StatefulSet") {
		statefulSet := appsv1.StatefulSet{}
		if err := k.Client.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, &statefulSet); err == nil {
			payload := workloadSnapshotPayload{
				Kind:                   "StatefulSet",
				Revision:               statefulSet.Status.CurrentRevision,
				StatefulSetTemplate:    statefulSet.Spec.Template.DeepCopy(),
				StatefulSetUpdate:      statefulSet.Spec.UpdateStrategy.DeepCopy(),
				StatefulSetAnnotations: cloneStringMap(statefulSet.Annotations),
			}
			if payload.Revision == "" {
				payload.Revision = "current"
			}
			return payload, payload.Revision, nil
		} else if !apierrors.IsNotFound(err) {
			return workloadSnapshotPayload{}, "", err
		}
	}
	return workloadSnapshotPayload{}, "", fmt.Errorf("workload %s/%s not found for snapshot", namespace, name)
}

func snapshotFromConfigMap(configMap corev1.ConfigMap) (Snapshot, error) {
	createdAt, err := time.Parse(time.RFC3339, configMap.Data["createdAt"])
	if err != nil {
		createdAt = configMap.CreationTimestamp.Time
	}
	expiresAt := time.Time{}
	if configMap.Data["expiresAt"] != "" {
		if parsed, parseErr := time.Parse(time.RFC3339, configMap.Data["expiresAt"]); parseErr == nil {
			expiresAt = parsed
		}
	}
	id := configMap.Annotations[snapshotIDAnnotation]
	if id == "" {
		id = configMap.Data["snapshotID"]
	}
	if id == "" {
		return Snapshot{}, fmt.Errorf("snapshot id is missing")
	}
	return Snapshot{
		ID:             id,
		ResourceName:   configMap.Name,
		Namespace:      configMap.Data["namespace"],
		Name:           configMap.Data["name"],
		WorkloadKind:   configMap.Data["workloadKind"],
		Revision:       configMap.Data["revision"],
		Phase:          configMap.Data["phase"],
		IdempotencyKey: configMap.Annotations[snapshotIdempotencyAnnotation],
		CreatedAt:      createdAt,
		ExpiresAt:      expiresAt,
	}, nil
}

func safeLabelValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "unknown"
	}
	replaced := strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(trimmed)
	if len(replaced) <= 63 {
		return replaced
	}
	return replaced[:50] + "-" + shortHash(replaced)[:12]
}

func shortHash(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func durationFromMinutes(minutes int, fallback int) time.Duration {
	if minutes < 1 {
		minutes = fallback
	}
	return time.Duration(minutes) * time.Minute
}

func (k *KubernetesSnapshotter) now() time.Time {
	if k.Now != nil {
		return k.Now().UTC()
	}
	return time.Now().UTC()
}

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}
