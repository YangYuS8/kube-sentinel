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

func Evaluate(input GateInput) GateDecision {
	if inMaintenanceWindow(input.Now, input.MaintenanceWindows) {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "maintenance window"}
	}
	if input.MaxActions < 1 {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "invalid rate limit config"}
	}
	if input.ActionsInWindow >= input.MaxActions {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "rate limit exceeded"}
	}
	if input.MaxPodPercentage < 1 || input.MaxPodPercentage > 100 {
		return GateDecision{Allow: false, ReadOnly: true, Reason: "invalid blast radius config"}
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
