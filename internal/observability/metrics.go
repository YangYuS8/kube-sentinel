package observability

import "sync/atomic"

type Metrics struct {
	Triggers      uint64
	Success       uint64
	Rollbacks     uint64
	CircuitBreaks uint64
}

func (m *Metrics) IncTriggers() { atomic.AddUint64(&m.Triggers, 1) }
func (m *Metrics) IncSuccess()  { atomic.AddUint64(&m.Success, 1) }
func (m *Metrics) IncRollbacks() {
	atomic.AddUint64(&m.Rollbacks, 1)
}
func (m *Metrics) IncCircuitBreaks() {
	atomic.AddUint64(&m.CircuitBreaks, 1)
}
