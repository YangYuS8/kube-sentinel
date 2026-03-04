package scripts

import (
	"testing"
	"time"
)

func TestComputeDeliveryTrendMetrics(t *testing.T) {
	start := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 4, 23, 59, 59, 0, time.UTC)
	samples := []DeliveryTrendSample{
		{Timestamp: start.Add(1 * time.Hour), GateOutcome: "allow", RecoverySeconds: 60, DrillExecuted: true},
		{Timestamp: start.Add(2 * time.Hour), GateOutcome: "block", RecoverySeconds: 180, DrillExecuted: false},
		{Timestamp: start.Add(3 * time.Hour), GateOutcome: "degrade", RecoverySeconds: 0, DrillExecuted: true},
	}

	metrics, err := ComputeDeliveryTrendMetrics(samples, start, end)
	if err != nil {
		t.Fatalf("compute metrics failed: %v", err)
	}
	if metrics.SampleCount != 3 {
		t.Fatalf("expected 3 samples, got %d", metrics.SampleCount)
	}
	if metrics.PassCount != 1 || metrics.BlockCount != 1 {
		t.Fatalf("unexpected pass/block count: %+v", metrics)
	}
	if metrics.PassRate <= 0.33 || metrics.PassRate >= 0.34 {
		t.Fatalf("unexpected pass rate: %f", metrics.PassRate)
	}
	if metrics.BlockRate <= 0.33 || metrics.BlockRate >= 0.34 {
		t.Fatalf("unexpected block rate: %f", metrics.BlockRate)
	}
	if metrics.RecoverySampleCount != 2 {
		t.Fatalf("expected 2 recovery samples, got %d", metrics.RecoverySampleCount)
	}
	if metrics.RecoveryMeanSeconds != 120 {
		t.Fatalf("unexpected recovery mean: %f", metrics.RecoveryMeanSeconds)
	}
	if metrics.DrillCoverageRate <= 0.66 || metrics.DrillCoverageRate >= 0.67 {
		t.Fatalf("unexpected drill coverage: %f", metrics.DrillCoverageRate)
	}
}

func TestComputeDeliveryTrendMetricsWindowBoundary(t *testing.T) {
	start := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 4, 1, 0, 0, 0, time.UTC)
	samples := []DeliveryTrendSample{
		{Timestamp: start, GateOutcome: "allow", RecoverySeconds: 30, DrillExecuted: true},
		{Timestamp: end, GateOutcome: "block", RecoverySeconds: 90, DrillExecuted: false},
		{Timestamp: end.Add(1 * time.Second), GateOutcome: "allow", RecoverySeconds: 10, DrillExecuted: true},
	}

	metrics, err := ComputeDeliveryTrendMetrics(samples, start, end)
	if err != nil {
		t.Fatalf("compute metrics failed: %v", err)
	}
	if metrics.SampleCount != 2 {
		t.Fatalf("expected boundary inclusive 2 samples, got %d", metrics.SampleCount)
	}
}

func TestComputeDeliveryTrendMetricsEmptyWindow(t *testing.T) {
	start := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 4, 0, 1, 0, 0, time.UTC)
	samples := []DeliveryTrendSample{{Timestamp: end.Add(2 * time.Minute), GateOutcome: "allow", RecoverySeconds: 10, DrillExecuted: true}}

	metrics, err := ComputeDeliveryTrendMetrics(samples, start, end)
	if err != nil {
		t.Fatalf("compute metrics failed: %v", err)
	}
	if metrics.SampleCount != 0 {
		t.Fatalf("expected empty window sample count 0, got %d", metrics.SampleCount)
	}
	if metrics.PassRate != 0 || metrics.BlockRate != 0 || metrics.DrillCoverageRate != 0 {
		t.Fatalf("expected zero rates on empty window: %+v", metrics)
	}
}

func TestComputeDeliveryTrendMetricsInvalidInput(t *testing.T) {
	start := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 4, 1, 0, 0, 0, time.UTC)

	if _, err := ComputeDeliveryTrendMetrics(nil, end, start); err == nil {
		t.Fatalf("expected invalid window to fail")
	}
	if _, err := ComputeDeliveryTrendMetrics([]DeliveryTrendSample{{Timestamp: start, GateOutcome: "allow", RecoverySeconds: -1}}, start, end); err == nil {
		t.Fatalf("expected negative recovery to fail")
	}
}
