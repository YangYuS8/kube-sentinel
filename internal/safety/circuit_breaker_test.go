package safety

import (
	"testing"
	"time"
)

func TestObjectBreakerOpenAndRecover(t *testing.T) {
	b := NewCircuitBreaker(2, 100, 1)
	now := time.Now()
	b.RecordFailure("ns/app", now)
	b.RecordFailure("ns/app", now)
	allow, _ := b.Allow("ns/app", now)
	if allow {
		t.Fatalf("expected object breaker open")
	}
	allow2, _ := b.Allow("ns/app", now.Add(2*time.Minute))
	if !allow2 {
		t.Fatalf("expected breaker recovered after cooldown")
	}
}

func TestDomainBreakerPriority(t *testing.T) {
	b := NewCircuitBreaker(100, 2, 1)
	now := time.Now()
	b.RecordFailure("ns/app1", now)
	b.RecordFailure("ns/app2", now)
	allow, reason := b.Allow("ns/app3", now)
	if allow || reason != "domain breaker open" {
		t.Fatalf("expected domain breaker open")
	}
	status := b.Status("ns/app3")
	if status.OpenReason != "domain breaker open" || status.RecoveryAt == "" {
		t.Fatalf("expected domain breaker evidence in status")
	}
}
