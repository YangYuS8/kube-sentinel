package safety

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type GateInput struct {
	Now                time.Time
	MaintenanceWindows []string
	ActionsInWindow    int
	MaxActions         int
	AffectedPods       int
	ClusterPods        int
	MaxPodPercentage   int
}

type GateDecision struct {
	Allow    bool
	ReadOnly bool
	Reason   string
}

type GateValidationResult struct {
	Valid      bool
	ReasonCode string
	Reason     string
}

func ValidateGateInput(input GateInput) GateValidationResult {
	if input.MaxActions < 1 {
		return GateValidationResult{Valid: false, ReasonCode: "invalid_rate_limit", Reason: "max actions must be >= 1"}
	}
	if input.MaxPodPercentage < 1 || input.MaxPodPercentage > 100 {
		return GateValidationResult{Valid: false, ReasonCode: "invalid_blast_radius", Reason: "max pod percentage must be between 1 and 100"}
	}
	if input.ClusterPods < 0 || input.AffectedPods < 0 {
		return GateValidationResult{Valid: false, ReasonCode: "invalid_workload_counts", Reason: "pod counts must be >= 0"}
	}
	if input.ClusterPods > 0 && input.AffectedPods > input.ClusterPods {
		return GateValidationResult{Valid: false, ReasonCode: "invalid_blast_radius_counts", Reason: "affected pods cannot exceed cluster pods"}
	}
	return GateValidationResult{Valid: true, ReasonCode: "ok"}
}

func Evaluate(input GateInput) GateDecision {
	validation := ValidateGateInput(input)
	if !validation.Valid {
		if validation.ReasonCode == "invalid_rate_limit" {
			return GateDecision{Allow: false, ReadOnly: true, Reason: "invalid rate limit config"}
		}
		return GateDecision{Allow: false, ReadOnly: true, Reason: "invalid blast radius config"}
	}
	if inMaintenanceWindow(input.Now, input.MaintenanceWindows) {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "maintenance window"}
	}
	if input.ActionsInWindow >= input.MaxActions {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "rate limit exceeded"}
	}
	if input.ClusterPods > 0 {
		pct := (input.AffectedPods * 100) / input.ClusterPods
		if pct > input.MaxPodPercentage {
			return GateDecision{Allow: false, ReadOnly: true, Reason: "blast radius exceeded"}
		}
	}
	return GateDecision{Allow: true}
}

func inMaintenanceWindow(now time.Time, windows []string) bool {
	for _, w := range windows {
		start, end, err := parseWindow(w)
		if err != nil {
			continue
		}
		curr := now.Hour()*60 + now.Minute()
		if start <= end {
			if curr >= start && curr <= end {
				return true
			}
			continue
		}
		if curr >= start || curr <= end {
			return true
		}
	}
	return false
}

func parseWindow(raw string) (int, int, error) {
	parts := strings.Split(raw, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid window")
	}
	start, err := parseHHMM(parts[0])
	if err != nil {
		return 0, 0, err
	}
	end, err := parseHHMM(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

func parseHHMM(raw string) (int, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid hh:mm")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("out of range")
	}
	return h*60 + m, nil
}
