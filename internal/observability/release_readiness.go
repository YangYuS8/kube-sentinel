package observability

import (
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DecisionAllow   = "allow"
	DecisionDegrade = "degrade"
	DecisionBlock   = "block"
)

type OperatorOverride struct {
	Enabled          bool      `json:"enabled"`
	Actor            string    `json:"actor,omitempty"`
	At               time.Time `json:"at,omitempty"`
	PreviousDecision string    `json:"previousDecision,omitempty"`
	NewDecision      string    `json:"newDecision,omitempty"`
	Reason           string    `json:"reason,omitempty"`
}

type IncidentRecord struct {
	ID        string    `json:"id"`
	Category  string    `json:"category,omitempty"`
	Source    string    `json:"source,omitempty"`
	IsDrill   bool      `json:"isDrill"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
}

type DrillAggregate struct {
	SuccessRate          float64 `json:"successRate"`
	RollbackLatencyP95MS int     `json:"rollbackLatencyP95Ms"`
	GateBypassCount      int     `json:"gateBypassCount"`
	RecentScore          float64 `json:"recentScore"`
}

type OnCallActionTemplate struct {
	Decision                 string `json:"decision"`
	AlertLevel               string `json:"alertLevel"`
	Runbook                  string `json:"runbook"`
	ApprovalTrigger          string `json:"approvalTrigger"`
	RollbackAction           string `json:"rollbackAction"`
	ObservationWindowMinutes int    `json:"observationWindowMinutes"`
}

type ReleaseReadinessInput struct {
	ActionType        string
	RiskLevel         string
	StrategyMode      string
	CircuitTier       string
	Decision          string
	RollbackCandidate string
	OpenIncidents     []IncidentRecord
	OperatorOverride  OperatorOverride
	Drill             DrillAggregate
}

type ReleaseReadinessSummary struct {
	ActionType        string               `json:"actionType"`
	RiskLevel         string               `json:"riskLevel"`
	StrategyMode      string               `json:"strategyMode"`
	CircuitTier       string               `json:"circuitTier"`
	Decision          string               `json:"decision"`
	OperatorOverride  OperatorOverride     `json:"operatorOverride"`
	RollbackCandidate string               `json:"rollbackCandidate"`
	OpenIncidents     []IncidentRecord     `json:"openIncidents"`
	RecentDrillScore  float64              `json:"recentDrillScore"`
	Drill             DrillAggregate       `json:"drill"`
	OnCallTemplate    OnCallActionTemplate `json:"onCallTemplate"`
}

func BuildReleaseReadinessSummary(input ReleaseReadinessInput) (ReleaseReadinessSummary, error) {
	decision := normalizeDecision(input.Decision)
	template, mapped := MapOnCallTemplate(decision)
	if !mapped {
		template, _ = MapOnCallTemplate(DecisionDegrade)
		decision = DecisionDegrade
	}

	incidents := dedupeIncidents(input.OpenIncidents)
	sort.SliceStable(incidents, func(i, j int) bool {
		if incidents[i].CreatedAt.Equal(incidents[j].CreatedAt) {
			return incidents[i].ID < incidents[j].ID
		}
		return incidents[i].CreatedAt.After(incidents[j].CreatedAt)
	})

	summary := ReleaseReadinessSummary{
		ActionType:        normalizeFreeText(input.ActionType, "unknown"),
		RiskLevel:         normalizeRisk(input.RiskLevel),
		StrategyMode:      normalizeFreeText(input.StrategyMode, "unknown"),
		CircuitTier:       normalizeFreeText(input.CircuitTier, "none"),
		Decision:          decision,
		OperatorOverride:  input.OperatorOverride,
		RollbackCandidate: strings.TrimSpace(input.RollbackCandidate),
		OpenIncidents:     incidents,
		RecentDrillScore:  normalizeScore(input.Drill.RecentScore),
		Drill: DrillAggregate{
			SuccessRate:          normalizeScore(input.Drill.SuccessRate),
			RollbackLatencyP95MS: maxInt(input.Drill.RollbackLatencyP95MS, 0),
			GateBypassCount:      maxInt(input.Drill.GateBypassCount, 0),
			RecentScore:          normalizeScore(input.Drill.RecentScore),
		},
		OnCallTemplate: template,
	}
	if summary.OperatorOverride.Enabled {
		summary.OperatorOverride.PreviousDecision = normalizeDecision(summary.OperatorOverride.PreviousDecision)
		summary.OperatorOverride.NewDecision = normalizeDecision(summary.OperatorOverride.NewDecision)
		if summary.OperatorOverride.NewDecision == "unknown" {
			summary.OperatorOverride.NewDecision = decision
		}
	}

	var buildErr error
	if strings.TrimSpace(input.ActionType) == "" {
		buildErr = errors.New("actionType is required and was defaulted to unknown")
	}
	if strings.TrimSpace(input.RollbackCandidate) == "" {
		if buildErr == nil {
			buildErr = errors.New("rollbackCandidate is empty; summary remains degradable")
		}
	}
	return summary, buildErr
}

func (s ReleaseReadinessSummary) Serialize() string {
	payload, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func (s ReleaseReadinessSummary) EnforceProductionGate(maxOpenIncidents int) (decision, reasonCode string) {
	if maxOpenIncidents <= 0 {
		maxOpenIncidents = 3
	}
	if strings.TrimSpace(s.RollbackCandidate) == "" {
		return DecisionBlock, "release_readiness_missing_rollback_candidate"
	}
	if len(s.OpenIncidents) > maxOpenIncidents {
		return DecisionBlock, "release_readiness_open_incidents_exceeded"
	}
	return s.Decision, ""
}

func MapOnCallTemplate(decision string) (OnCallActionTemplate, bool) {
	switch normalizeDecision(decision) {
	case DecisionAllow:
		return OnCallActionTemplate{
			Decision:                 DecisionAllow,
			AlertLevel:               "info",
			Runbook:                  "runbook://runtime-observation",
			ApprovalTrigger:          "none",
			RollbackAction:           "observe",
			ObservationWindowMinutes: 30,
		}, true
	case DecisionDegrade:
		return OnCallActionTemplate{
			Decision:                 DecisionDegrade,
			AlertLevel:               "warning",
			Runbook:                  "runbook://runtime-degrade-recovery",
			ApprovalTrigger:          "oncall-ack",
			RollbackAction:           "prepare-rollback",
			ObservationWindowMinutes: 15,
		}, true
	case DecisionBlock:
		return OnCallActionTemplate{
			Decision:                 DecisionBlock,
			AlertLevel:               "critical",
			Runbook:                  "runbook://runtime-block-rollback",
			ApprovalTrigger:          "incident-commander",
			RollbackAction:           "execute-rollback",
			ObservationWindowMinutes: 5,
		}, true
	default:
		return OnCallActionTemplate{}, false
	}
}

func BuildIncidentsFromCSV(raw string, isDrill bool, source string, now time.Time) []IncidentRecord {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	records := make([]IncidentRecord, 0, len(parts))
	for _, token := range parts {
		id := strings.TrimSpace(token)
		if id == "" {
			continue
		}
		records = append(records, IncidentRecord{ID: id, Source: source, IsDrill: isDrill, CreatedAt: now})
	}
	return dedupeIncidents(records)
}

func ParseDrillAggregate(successRateRaw, latencyRaw, bypassRaw, scoreRaw string) DrillAggregate {
	successRate, _ := strconv.ParseFloat(strings.TrimSpace(successRateRaw), 64)
	latency, _ := strconv.Atoi(strings.TrimSpace(latencyRaw))
	bypass, _ := strconv.Atoi(strings.TrimSpace(bypassRaw))
	score, _ := strconv.ParseFloat(strings.TrimSpace(scoreRaw), 64)
	return DrillAggregate{
		SuccessRate:          normalizeScore(successRate),
		RollbackLatencyP95MS: maxInt(latency, 0),
		GateBypassCount:      maxInt(bypass, 0),
		RecentScore:          normalizeScore(score),
	}
}

func normalizeDecision(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case DecisionAllow:
		return DecisionAllow
	case DecisionDegrade:
		return DecisionDegrade
	case DecisionBlock:
		return DecisionBlock
	default:
		return "unknown"
	}
}

func normalizeRisk(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "low", "medium", "high":
		return strings.TrimSpace(strings.ToLower(raw))
	default:
		return "medium"
	}
}

func normalizeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func normalizeFreeText(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	return value
}

func dedupeIncidents(records []IncidentRecord) []IncidentRecord {
	if len(records) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]IncidentRecord, 0, len(records))
	for _, item := range records {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		key := id + "|" + strings.TrimSpace(item.Source) + "|" + strconv.FormatBool(item.IsDrill)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		item.ID = id
		result = append(result, item)
	}
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
