package safety

import (
	"encoding/json"
	"fmt"
	"sync"
)

type GateOutcome string

type GateReasonCode string

const (
	GateOutcomeAllow   GateOutcome = "allow"
	GateOutcomeBlock   GateOutcome = "block"
	GateOutcomeDegrade GateOutcome = "degrade"
)

const (
	ReasonCodeMaintenanceWindow GateReasonCode = "maintenance_window"
	ReasonCodeSecurityGate      GateReasonCode = "security_gate"
	ReasonCodeCircuitBreaker    GateReasonCode = "circuit_breaker_open"
	ReasonCodeReleaseThreshold  GateReasonCode = "release_threshold_exceeded"
	ReasonCodeRateLimit         GateReasonCode = "rate_limit_exceeded"
	ReasonCodeBlastRadius       GateReasonCode = "blast_radius_exceeded"
	ReasonCodeInvalidConfig     GateReasonCode = "invalid_gate_config"
	ReasonCodeDataIncomplete    GateReasonCode = "data_incomplete"
)

type GateThresholdSnapshot struct {
	ActionsInWindow      int     `json:"actionsInWindow,omitempty"`
	MaxActions           int     `json:"maxActions,omitempty"`
	AffectedPods         int     `json:"affectedPods,omitempty"`
	ClusterPods          int     `json:"clusterPods,omitempty"`
	MaxPodPercentage     int     `json:"maxPodPercentage,omitempty"`
	L1SuccessRatePercent float64 `json:"l1SuccessRatePercent,omitempty"`
	L2SuccessRatePercent float64 `json:"l2SuccessRatePercent,omitempty"`
	L3DegradeRatePercent float64 `json:"l3DegradeRatePercent,omitempty"`
	BlockRatePercent     float64 `json:"blockRatePercent,omitempty"`
}

type GateEvidence struct {
	ReasonCode     GateReasonCode        `json:"reasonCode,omitempty"`
	SampleWindow   string                `json:"sampleWindow,omitempty"`
	KeyMetrics     map[string]float64    `json:"keyMetrics,omitempty"`
	Thresholds     GateThresholdSnapshot `json:"thresholds,omitempty"`
	Recommendation string                `json:"recommendation,omitempty"`
	DataComplete   bool                  `json:"dataComplete"`
}

func (e *GateEvidence) Normalize() {
	e.DataComplete = e.ReasonCode != "" && e.SampleWindow != "" && e.Recommendation != "" && len(e.KeyMetrics) > 0
}

func (e GateEvidence) MarshalJSON() ([]byte, error) {
	type alias GateEvidence
	out := alias(e)
	if out.ReasonCode == "" || out.SampleWindow == "" || out.Recommendation == "" || len(out.KeyMetrics) == 0 {
		out.DataComplete = false
	}
	return json.Marshal(out)
}

type ConservativeAction struct {
	ReadOnly           bool   `json:"readOnly"`
	EmitAlert          bool   `json:"emitAlert"`
	ManualIntervention bool   `json:"manualIntervention"`
	Recommendation     string `json:"recommendation,omitempty"`
}

type ProductionGateDecision struct {
	Outcome    GateOutcome        `json:"outcome"`
	ReasonCode GateReasonCode     `json:"reasonCode"`
	Reason     string             `json:"reason"`
	Evidence   GateEvidence       `json:"evidence"`
	Action     ConservativeAction `json:"action"`
}

type ProductionGateInput struct {
	Gate                     GateInput
	MaintenanceWindowHit     bool
	SecurityGateHit          bool
	CircuitBreakerOpen       bool
	ReleaseThresholdExceeded bool
	SampleWindow             string
	KeyMetrics               map[string]float64
	Thresholds               GateThresholdSnapshot
}

func EvaluateProductionGate(input ProductionGateInput) ProductionGateDecision {
	if input.MaintenanceWindowHit {
		return blockDecision(ReasonCodeMaintenanceWindow, "maintenance window active", input)
	}
	if input.SecurityGateHit {
		return blockDecision(ReasonCodeSecurityGate, "security gate blocked", input)
	}
	if input.CircuitBreakerOpen {
		return blockDecision(ReasonCodeCircuitBreaker, "circuit breaker open", input)
	}
	if input.ReleaseThresholdExceeded {
		return degradeDecision(ReasonCodeReleaseThreshold, "release threshold exceeded", input)
	}
	legacyDecision := Evaluate(input.Gate)
	if legacyDecision.Allow {
		return allowDecision(input)
	}
	switch legacyDecision.Reason {
	case "maintenance window":
		return blockDecision(ReasonCodeMaintenanceWindow, legacyDecision.Reason, input)
	case "invalid rate limit config", "invalid blast radius config":
		return safeBlockDecision(input)
	case "rate limit exceeded":
		return degradeDecision(ReasonCodeRateLimit, legacyDecision.Reason, input)
	case "blast radius exceeded":
		return degradeDecision(ReasonCodeBlastRadius, legacyDecision.Reason, input)
	default:
		return safeBlockDecision(input)
	}
}

func allowDecision(input ProductionGateInput) ProductionGateDecision {
	evidence := GateEvidence{
		ReasonCode:     "",
		SampleWindow:   input.SampleWindow,
		KeyMetrics:     cloneMetrics(input.KeyMetrics),
		Thresholds:     input.Thresholds,
		Recommendation: "continue rollout with routine observation",
	}
	evidence.Normalize()
	return ProductionGateDecision{
		Outcome:    GateOutcomeAllow,
		ReasonCode: "",
		Reason:     "allowed",
		Evidence:   evidence,
		Action: ConservativeAction{
			ReadOnly:           false,
			EmitAlert:          false,
			ManualIntervention: false,
		},
	}
}

func blockDecision(code GateReasonCode, reason string, input ProductionGateInput) ProductionGateDecision {
	recommendation := "switch to conservative mode and request manual confirmation"
	evidence := GateEvidence{
		ReasonCode:     code,
		SampleWindow:   input.SampleWindow,
		KeyMetrics:     cloneMetrics(input.KeyMetrics),
		Thresholds:     input.Thresholds,
		Recommendation: recommendation,
	}
	evidence.Normalize()
	if !evidence.DataComplete {
		return safeBlockDecision(input)
	}
	return ProductionGateDecision{
		Outcome:    GateOutcomeBlock,
		ReasonCode: code,
		Reason:     reason,
		Evidence:   evidence,
		Action: ConservativeAction{
			ReadOnly:           true,
			EmitAlert:          true,
			ManualIntervention: true,
			Recommendation:     recommendation,
		},
	}
}

func degradeDecision(code GateReasonCode, reason string, input ProductionGateInput) ProductionGateDecision {
	recommendation := "enter read-only evaluation, alert on-call, and prepare rollback"
	evidence := GateEvidence{
		ReasonCode:     code,
		SampleWindow:   input.SampleWindow,
		KeyMetrics:     cloneMetrics(input.KeyMetrics),
		Thresholds:     input.Thresholds,
		Recommendation: recommendation,
	}
	evidence.Normalize()
	if !evidence.DataComplete {
		return safeBlockDecision(input)
	}
	return ProductionGateDecision{
		Outcome:    GateOutcomeDegrade,
		ReasonCode: code,
		Reason:     reason,
		Evidence:   evidence,
		Action: ConservativeAction{
			ReadOnly:           true,
			EmitAlert:          true,
			ManualIntervention: true,
			Recommendation:     recommendation,
		},
	}
}

func safeBlockDecision(input ProductionGateInput) ProductionGateDecision {
	evidence := GateEvidence{
		ReasonCode:     ReasonCodeDataIncomplete,
		SampleWindow:   input.SampleWindow,
		KeyMetrics:     cloneMetrics(input.KeyMetrics),
		Thresholds:     input.Thresholds,
		Recommendation: "fallback to conservative mode until gate evidence is complete",
		DataComplete:   false,
	}
	return ProductionGateDecision{
		Outcome:    GateOutcomeBlock,
		ReasonCode: ReasonCodeDataIncomplete,
		Reason:     "insufficient evidence; conservative block",
		Evidence:   evidence,
		Action: ConservativeAction{
			ReadOnly:           true,
			EmitAlert:          true,
			ManualIntervention: true,
			Recommendation:     evidence.Recommendation,
		},
	}
}

func cloneMetrics(metrics map[string]float64) map[string]float64 {
	if len(metrics) == 0 {
		return nil
	}
	out := make(map[string]float64, len(metrics))
	for key, value := range metrics {
		out[key] = value
	}
	return out
}

type ConservativeModeState struct {
	ObjectKey      string
	Outcome        GateOutcome
	ReasonCode     GateReasonCode
	Recommendation string
	ReadOnly       bool
	AlertEmitted   bool
	ManualRequired bool
}

type ConservativeModeExecutor struct {
	mu      sync.Mutex
	active  map[string]ConservativeModeState
	history map[string][]ConservativeModeState
}

type ConservativeModeSnapshot struct {
	State  ConservativeModeState
	Exists bool
}

func NewConservativeModeExecutor() *ConservativeModeExecutor {
	return &ConservativeModeExecutor{
		active:  map[string]ConservativeModeState{},
		history: map[string][]ConservativeModeState{},
	}
}

func (e *ConservativeModeExecutor) Apply(objectKey string, decision ProductionGateDecision) (ConservativeModeState, bool, error) {
	if objectKey == "" {
		return ConservativeModeState{}, false, fmt.Errorf("object key is required")
	}
	if decision.Outcome != GateOutcomeBlock && decision.Outcome != GateOutcomeDegrade {
		return ConservativeModeState{}, false, fmt.Errorf("conservative mode can only be applied for block/degrade outcomes")
	}
	next := ConservativeModeState{
		ObjectKey:      objectKey,
		Outcome:        decision.Outcome,
		ReasonCode:     decision.ReasonCode,
		Recommendation: decision.Action.Recommendation,
		ReadOnly:       decision.Action.ReadOnly,
		AlertEmitted:   decision.Action.EmitAlert,
		ManualRequired: decision.Action.ManualIntervention,
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	current, exists := e.active[objectKey]
	if exists && current == next {
		return current, false, nil
	}
	if exists {
		e.history[objectKey] = append(e.history[objectKey], current)
	}
	e.active[objectKey] = next
	return next, true, nil
}

func (e *ConservativeModeExecutor) Rollback(objectKey string) (ConservativeModeState, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	history := e.history[objectKey]
	if len(history) == 0 {
		delete(e.active, objectKey)
		return ConservativeModeState{}, false
	}
	last := history[len(history)-1]
	e.history[objectKey] = history[:len(history)-1]
	e.active[objectKey] = last
	return last, true
}

func (e *ConservativeModeExecutor) Snapshot(objectKey string) ConservativeModeSnapshot {
	e.mu.Lock()
	defer e.mu.Unlock()
	state, exists := e.active[objectKey]
	return ConservativeModeSnapshot{State: state, Exists: exists}
}

func (e *ConservativeModeExecutor) Restore(objectKey string, snapshot ConservativeModeSnapshot) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !snapshot.Exists {
		delete(e.active, objectKey)
		return
	}
	e.active[objectKey] = snapshot.State
}

func (e *ConservativeModeExecutor) CanAutoEscalate(objectKey string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	state, exists := e.active[objectKey]
	if !exists {
		return true
	}
	return !state.ManualRequired
}
