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
	strategyDurationHistogram = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kube_sentinel_strategy_duration_seconds",
		Help:    "Duration of healing strategy execution.",
		Buckets: prometheus.DefBuckets,
	}, []string{"stage"})
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
			strategyDurationHistogram,
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

func (m *Metrics) ObserveStrategyDuration(stage string, duration time.Duration) {
	registerPrometheusMetrics()
	strategyDurationHistogram.WithLabelValues(stage).Observe(duration.Seconds())
}
