package observability

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	Triggers                   uint64
	Success                    uint64
	Failures                   uint64
	Rollbacks                  uint64
	CircuitBreaks              uint64
	MaintenanceWindowConflicts uint64
	Suppressed                 uint64
	ReadOnlyBlocks             uint64
	StatefulSetFreezeTriggers  uint64
	StatefulSetL2Successes     uint64
	StatefulSetL2Fallbacks     uint64
	StatefulSetL2Degrades      uint64
	SnapshotCreateSuccesses    uint64
	SnapshotCreateFailures     uint64
	SnapshotRestoreSuccesses   uint64
	SnapshotRestoreFailures    uint64
	SnapshotCapacityBlocks     uint64
	SnapshotPruned             uint64
	DeploymentL1Successes      uint64
	DeploymentL1Failures       uint64
	DeploymentL1Blocks         uint64
	DeploymentL2Successes      uint64
	DeploymentL2Fallbacks      uint64
	DeploymentL2Degrades       uint64
	DeploymentStageBlocks      uint64
	ProductionGateReports      uint64
	GateReportMissingFields    uint64
}

var (
	registerMetricsOnce sync.Once

	triggersCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_triggers_total",
		Help: "Total number of healing triggers.",
	})
	successCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_success_total",
		Help: "Total number of successful healing actions.",
	})
	failuresCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_healing_failures_total",
		Help: "Total number of failed healing actions.",
	})
	rollbacksCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_rollbacks_total",
		Help: "Total number of rollbacks.",
	})
	circuitBreaksCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_circuit_breaks_total",
		Help: "Total number of circuit breaker triggers.",
	})
	maintenanceWindowConflictsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_maintenance_window_conflict_total",
		Help: "Total number of blocked actions due to maintenance windows.",
	})
	suppressedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_suppressed_total",
		Help: "Total number of suppressed actions during soak verification.",
	})
	readOnlyBlocksCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_readonly_blocks_total",
		Help: "Total number of read-only blocked actions by reason.",
	}, []string{"reason", "workload_kind"})
	statefulSetControlledActionCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_statefulset_controlled_actions_total",
		Help: "Total number of StatefulSet controlled action decisions.",
	}, []string{"workload_kind", "action_type", "decision", "freeze_state"})
	statefulSetFreezeTriggersCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_statefulset_freeze_triggers_total",
		Help: "Total number of StatefulSet freeze triggers.",
	})
	statefulSetL2ResultCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_statefulset_l2_results_total",
		Help: "Total number of StatefulSet L2 rollback results by result type.",
	}, []string{"result"})
	snapshotCreateCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_snapshot_creates_total",
		Help: "Total number of snapshot create attempts by result.",
	}, []string{"result"})
	snapshotRestoreCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_snapshot_restores_total",
		Help: "Total number of snapshot restore attempts by result.",
	}, []string{"result"})
	snapshotCapacityBlocksCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_snapshot_capacity_blocks_total",
		Help: "Total number of blocked actions due to snapshot capacity limits.",
	})
	snapshotPrunedCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_snapshot_pruned_total",
		Help: "Total number of pruned snapshots.",
	})
	snapshotActiveGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "kube_sentinel_snapshot_active",
		Help: "Current number of active snapshots for the recently processed workload.",
	})
	snapshotRestoreDurationHistogram = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "kube_sentinel_snapshot_restore_duration_seconds",
		Help:    "Duration of snapshot restore execution.",
		Buckets: prometheus.DefBuckets,
	})
	strategyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kube_sentinel_strategy_duration_seconds",
		Help:    "Duration of healing strategy execution.",
		Buckets: prometheus.DefBuckets,
	}, []string{"stage"})
	deploymentL1ResultCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_deployment_l1_results_total",
		Help: "Total number of Deployment L1 action results by result type.",
	}, []string{"result"})
	deploymentL2ResultCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_deployment_l2_results_total",
		Help: "Total number of Deployment L2 action results by result type.",
	}, []string{"result"})
	deploymentStageBlocksCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "kube_sentinel_deployment_stage_blocks_total",
		Help: "Total number of Deployment stage blocks by reason.",
	}, []string{"reason"})
	productionGateReportsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_production_gate_reports_total",
		Help: "Total number of emitted production gate reports.",
	})
	productionGateReportMissingFieldsCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "kube_sentinel_production_gate_report_missing_fields_total",
		Help: "Total number of production gate reports missing required fields.",
	})
)

func registerPrometheusMetrics() {
	registerMetricsOnce.Do(func() {
		prometheus.MustRegister(
			triggersCounter,
			successCounter,
			failuresCounter,
			rollbacksCounter,
			circuitBreaksCounter,
			maintenanceWindowConflictsCounter,
			suppressedCounter,
			readOnlyBlocksCounter,
			statefulSetControlledActionCounter,
			statefulSetFreezeTriggersCounter,
			statefulSetL2ResultCounter,
			snapshotCreateCounter,
			snapshotRestoreCounter,
			snapshotCapacityBlocksCounter,
			snapshotPrunedCounter,
			snapshotActiveGauge,
			snapshotRestoreDurationHistogram,
			strategyDurationHistogram,
			deploymentL1ResultCounter,
			deploymentL2ResultCounter,
			deploymentStageBlocksCounter,
			productionGateReportsCounter,
			productionGateReportMissingFieldsCounter,
		)
	})
}

func (m *Metrics) IncTriggers() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.Triggers, 1)
	triggersCounter.Inc()
}
func (m *Metrics) IncSuccess() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.Success, 1)
	successCounter.Inc()
}
func (m *Metrics) IncFailures() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.Failures, 1)
	failuresCounter.Inc()
}
func (m *Metrics) IncRollbacks() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.Rollbacks, 1)
	rollbacksCounter.Inc()
}
func (m *Metrics) IncCircuitBreaks() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.CircuitBreaks, 1)
	circuitBreaksCounter.Inc()
}
func (m *Metrics) IncMaintenanceWindowConflicts() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.MaintenanceWindowConflicts, 1)
	maintenanceWindowConflictsCounter.Inc()
}

func (m *Metrics) IncSuppressed() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.Suppressed, 1)
	suppressedCounter.Inc()
}

func (m *Metrics) IncReadOnlyBlocks(reason, workloadKind string) {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.ReadOnlyBlocks, 1)
	if reason == "" {
		reason = "unknown"
	}
	if workloadKind == "" {
		workloadKind = "unknown"
	}
	readOnlyBlocksCounter.WithLabelValues(reason, workloadKind).Inc()
}

func (m *Metrics) IncStatefulSetControlledAction(workloadKind, actionType, decision, freezeState string) {
	registerPrometheusMetrics()
	if workloadKind == "" {
		workloadKind = "unknown"
	}
	if actionType == "" {
		actionType = "unknown"
	}
	if decision == "" {
		decision = "unknown"
	}
	if freezeState == "" {
		freezeState = "none"
	}
	statefulSetControlledActionCounter.WithLabelValues(workloadKind, actionType, decision, freezeState).Inc()
}

func (m *Metrics) IncStatefulSetFreezeTriggers() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.StatefulSetFreezeTriggers, 1)
	statefulSetFreezeTriggersCounter.Inc()
}

func (m *Metrics) IncStatefulSetL2Result(result string) {
	registerPrometheusMetrics()
	if result == "" {
		result = "unknown"
	}
	switch result {
	case "success":
		atomic.AddUint64(&m.StatefulSetL2Successes, 1)
	case "fallback":
		atomic.AddUint64(&m.StatefulSetL2Fallbacks, 1)
	case "degraded":
		atomic.AddUint64(&m.StatefulSetL2Degrades, 1)
	}
	statefulSetL2ResultCounter.WithLabelValues(result).Inc()
}

func (m *Metrics) IncSnapshotCreateSuccess() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotCreateSuccesses, 1)
	snapshotCreateCounter.WithLabelValues("success").Inc()
}

func (m *Metrics) IncSnapshotCreateFailure() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotCreateFailures, 1)
	snapshotCreateCounter.WithLabelValues("failure").Inc()
}

func (m *Metrics) IncSnapshotRestoreSuccess() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotRestoreSuccesses, 1)
	snapshotRestoreCounter.WithLabelValues("success").Inc()
}

func (m *Metrics) IncSnapshotRestoreFailure() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotRestoreFailures, 1)
	snapshotRestoreCounter.WithLabelValues("failure").Inc()
}

func (m *Metrics) IncSnapshotCapacityBlock() {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotCapacityBlocks, 1)
	snapshotCapacityBlocksCounter.Inc()
}

func (m *Metrics) AddSnapshotPruned(count int) {
	if count <= 0 {
		return
	}
	registerPrometheusMetrics()
	atomic.AddUint64(&m.SnapshotPruned, uint64(count))
	snapshotPrunedCounter.Add(float64(count))
}

func (m *Metrics) SetSnapshotActive(count int) {
	registerPrometheusMetrics()
	if count < 0 {
		count = 0
	}
	snapshotActiveGauge.Set(float64(count))
}

func (m *Metrics) ObserveSnapshotRestoreDuration(duration time.Duration) {
	registerPrometheusMetrics()
	snapshotRestoreDurationHistogram.Observe(duration.Seconds())
}

func (m *Metrics) ObserveStrategyDuration(stage string, duration time.Duration) {
	registerPrometheusMetrics()
	strategyDurationHistogram.WithLabelValues(stage).Observe(duration.Seconds())
}

func (m *Metrics) IncDeploymentL1Result(result string) {
	registerPrometheusMetrics()
	if result == "" {
		result = "unknown"
	}
	switch result {
	case "success":
		atomic.AddUint64(&m.DeploymentL1Successes, 1)
	case "failed":
		atomic.AddUint64(&m.DeploymentL1Failures, 1)
	case "blocked":
		atomic.AddUint64(&m.DeploymentL1Blocks, 1)
	}
	deploymentL1ResultCounter.WithLabelValues(result).Inc()
}

func (m *Metrics) IncDeploymentL2Result(result string) {
	registerPrometheusMetrics()
	if result == "" {
		result = "unknown"
	}
	switch result {
	case "success":
		atomic.AddUint64(&m.DeploymentL2Successes, 1)
	case "fallback":
		atomic.AddUint64(&m.DeploymentL2Fallbacks, 1)
	case "degraded":
		atomic.AddUint64(&m.DeploymentL2Degrades, 1)
	}
	deploymentL2ResultCounter.WithLabelValues(result).Inc()
}

func (m *Metrics) IncDeploymentStageBlock(reason string) {
	registerPrometheusMetrics()
	if reason == "" {
		reason = "unknown"
	}
	atomic.AddUint64(&m.DeploymentStageBlocks, 1)
	deploymentStageBlocksCounter.WithLabelValues(reason).Inc()
}

func (m *Metrics) IncProductionGateReport(complete bool) {
	registerPrometheusMetrics()
	atomic.AddUint64(&m.ProductionGateReports, 1)
	productionGateReportsCounter.Inc()
	if !complete {
		atomic.AddUint64(&m.GateReportMissingFields, 1)
		productionGateReportMissingFieldsCounter.Inc()
	}
}

func (m *Metrics) DeploymentTieredRates() (l1SuccessRate, l2SuccessRate, l3DegradeRate, blockRate float64) {
	l1Success := atomic.LoadUint64(&m.DeploymentL1Successes)
	l1Failed := atomic.LoadUint64(&m.DeploymentL1Failures)
	l1Blocked := atomic.LoadUint64(&m.DeploymentL1Blocks)
	l2Success := atomic.LoadUint64(&m.DeploymentL2Successes)
	l2Fallback := atomic.LoadUint64(&m.DeploymentL2Fallbacks)
	l2Degraded := atomic.LoadUint64(&m.DeploymentL2Degrades)
	stageBlocked := atomic.LoadUint64(&m.DeploymentStageBlocks)

	l1Total := l1Success + l1Failed + l1Blocked
	if l1Total == 0 {
		l1SuccessRate = 100
	} else {
		l1SuccessRate = float64(l1Success) * 100 / float64(l1Total)
	}

	l2Total := l2Success + l2Fallback + l2Degraded
	if l2Total == 0 {
		l2SuccessRate = 100
		l3DegradeRate = 0
	} else {
		l2SuccessRate = float64(l2Success) * 100 / float64(l2Total)
		l3DegradeRate = float64(l2Degraded) * 100 / float64(l2Total)
	}

	totalStages := l1Total + l2Total
	if totalStages == 0 {
		blockRate = 0
	} else {
		blockRate = float64(stageBlocked) * 100 / float64(totalStages)
	}

	return
}
