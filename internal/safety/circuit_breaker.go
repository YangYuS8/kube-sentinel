package safety

import (
	"sync"
	"time"
)

type GateArbitrationInput struct {
	NamespaceBudgetBlocked bool
	CircuitBreakerOpen     bool
	ReleaseGateBlocked     bool
}

type GateArbitrationResult struct {
	Blocked    bool
	ReasonCode string
	Source     string
	Action     string
}

func ResolveGateArbitration(input GateArbitrationInput) GateArbitrationResult {
	if input.NamespaceBudgetBlocked {
		return GateArbitrationResult{
			Blocked:    true,
			ReasonCode: "namespace_budget_blocked",
			Source:     "namespace_budget",
			Action:     "readonly-block",
		}
	}
	if input.CircuitBreakerOpen {
		return GateArbitrationResult{
			Blocked:    true,
			ReasonCode: "circuit_breaker_open",
			Source:     "circuit_breaker",
			Action:     "readonly-block",
		}
	}
	if input.ReleaseGateBlocked {
		return GateArbitrationResult{
			Blocked:    true,
			ReasonCode: "release_gate_blocked",
			Source:     "release_gate",
			Action:     "degrade",
		}
	}
	return GateArbitrationResult{
		Blocked:    false,
		ReasonCode: "none",
		Source:     "allow",
		Action:     "allow",
	}
}

type breakerState struct {
	failures  int
	openUntil time.Time
}

type BreakerStatus struct {
	CurrentObjectFailures int
	CurrentDomainFailures int
	OpenReason            string
	RecoveryAt            string
}

type CircuitBreaker struct {
	mu              sync.Mutex
	objectThreshold int
	domainThreshold int
	cooldown        time.Duration
	objectStates    map[string]breakerState
	domainState     breakerState
}

func NewCircuitBreaker(objectThreshold, domainThreshold, cooldownMinutes int) *CircuitBreaker {
	return &CircuitBreaker{
		objectThreshold: objectThreshold,
		domainThreshold: domainThreshold,
		cooldown:        time.Duration(cooldownMinutes) * time.Minute,
		objectStates:    map[string]breakerState{},
	}
}

func (b *CircuitBreaker) Allow(objectKey string, now time.Time) (bool, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if now.Before(b.domainState.openUntil) {
		return false, "domain breaker open"
	}
	obj := b.objectStates[objectKey]
	if now.Before(obj.openUntil) {
		return false, "object breaker open"
	}
	return true, ""
}

func (b *CircuitBreaker) RecordFailure(objectKey string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	obj := b.objectStates[objectKey]
	obj.failures++
	if obj.failures >= b.objectThreshold {
		obj.openUntil = now.Add(b.cooldown)
		obj.failures = 0
	}
	b.objectStates[objectKey] = obj

	b.domainState.failures++
	if b.domainState.failures >= b.domainThreshold {
		b.domainState.openUntil = now.Add(b.cooldown)
		b.domainState.failures = 0
	}
}

func (b *CircuitBreaker) Recover(objectKey string, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	obj := b.objectStates[objectKey]
	if !now.Before(obj.openUntil) {
		obj.openUntil = time.Time{}
	}
	b.objectStates[objectKey] = obj
	if !now.Before(b.domainState.openUntil) {
		b.domainState.openUntil = time.Time{}
	}
}

func (b *CircuitBreaker) Status(objectKey string) BreakerStatus {
	b.mu.Lock()
	defer b.mu.Unlock()
	obj := b.objectStates[objectKey]
	status := BreakerStatus{
		CurrentObjectFailures: obj.failures,
		CurrentDomainFailures: b.domainState.failures,
	}
	if !obj.openUntil.IsZero() {
		status.OpenReason = "object breaker open"
		status.RecoveryAt = obj.openUntil.Format(time.RFC3339)
		return status
	}
	if !b.domainState.openUntil.IsZero() {
		status.OpenReason = "domain breaker open"
		status.RecoveryAt = b.domainState.openUntil.Format(time.RFC3339)
	}
	return status
}
