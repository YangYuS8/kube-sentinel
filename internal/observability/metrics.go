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

func (m *Metrics) ObserveStrategyDuration(stage string, duration time.Duration) {
	registerPrometheusMetrics()
	strategyDurationHistogram.WithLabelValues(stage).Observe(duration.Seconds())
}
