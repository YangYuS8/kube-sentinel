package scripts

import (
	"fmt"
	"time"
)

type DeliveryTrendSample struct {
	Timestamp       time.Time
	GateOutcome     string
	RecoverySeconds int64
	DrillExecuted   bool
}

type DeliveryTrendMetrics struct {
	WindowStart              time.Time
	WindowEnd                time.Time
	SampleCount              int
	PassRate                 float64
	BlockRate                float64
	RecoveryMeanSeconds      float64
	DrillCoverageRate        float64
	RecoverySampleCount      int
	PassCount                int
	BlockCount               int
	DrillExecutedSampleCount int
}

func normalizeTrendOutcome(outcome string) string {
	switch outcome {
	case "allow", "degrade", "block":
		return outcome
	default:
		return "degrade"
	}
}

func ComputeDeliveryTrendMetrics(samples []DeliveryTrendSample, windowStart, windowEnd time.Time) (DeliveryTrendMetrics, error) {
	if windowEnd.Before(windowStart) {
		return DeliveryTrendMetrics{}, fmt.Errorf("invalid window: end before start")
	}

	metrics := DeliveryTrendMetrics{
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}

	var recoverySum int64
	for _, sample := range samples {
		if sample.Timestamp.Before(windowStart) || sample.Timestamp.After(windowEnd) {
			continue
		}
		if sample.RecoverySeconds < 0 {
			return DeliveryTrendMetrics{}, fmt.Errorf("invalid recovery seconds: %d", sample.RecoverySeconds)
		}

		metrics.SampleCount++
		outcome := normalizeTrendOutcome(sample.GateOutcome)
		if outcome == "allow" {
			metrics.PassCount++
		}
		if outcome == "block" {
			metrics.BlockCount++
		}
		if sample.DrillExecuted {
			metrics.DrillExecutedSampleCount++
		}
		if sample.RecoverySeconds > 0 {
			recoverySum += sample.RecoverySeconds
			metrics.RecoverySampleCount++
		}
	}

	if metrics.SampleCount == 0 {
		return metrics, nil
	}

	metrics.PassRate = float64(metrics.PassCount) / float64(metrics.SampleCount)
	metrics.BlockRate = float64(metrics.BlockCount) / float64(metrics.SampleCount)
	metrics.DrillCoverageRate = float64(metrics.DrillExecutedSampleCount) / float64(metrics.SampleCount)
	if metrics.RecoverySampleCount > 0 {
		metrics.RecoveryMeanSeconds = float64(recoverySum) / float64(metrics.RecoverySampleCount)
	}

	return metrics, nil
}
