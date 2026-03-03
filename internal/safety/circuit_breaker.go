package safety

import (
	"sync"
	"time"
)

type breakerState struct {
	failures  int
	openUntil time.Time
}

type CircuitBreaker struct {
	mu               sync.Mutex
	objectThreshold  int
	domainThreshold  int
	cooldown         time.Duration
	objectStates     map[string]breakerState
	domainState      breakerState
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
