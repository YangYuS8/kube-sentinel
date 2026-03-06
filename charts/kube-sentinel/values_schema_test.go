package kube_sentinel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValuesSchemaIncludesProductionGatePolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.schema.json"))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	healingRequest := properties["healingRequest"].(map[string]interface{})
	healingProperties := healingRequest["properties"].(map[string]interface{})
	productionGatePolicy, ok := healingProperties["productionGatePolicy"].(map[string]interface{})
	if !ok {
		t.Fatalf("productionGatePolicy missing in healingRequest properties")
	}
	required := productionGatePolicy["required"].([]interface{})
	if len(required) != 2 || required[0].(string) != "sampleWindowMinutes" || required[1].(string) != "failureRatioBlockPercent" {
		t.Fatalf("unexpected productionGatePolicy required fields: %#v", required)
	}
}

func TestValuesYamlIncludesProductionGatePolicyDefaults(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.yaml"))
	if err != nil {
		t.Fatalf("read values yaml failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{"productionGatePolicy:", "sampleWindowMinutes: 10", "failureRatioBlockPercent: 30"} {
		if !strings.Contains(content, token) {
			t.Fatalf("values.yaml missing %s", token)
		}
	}
}

func TestValuesSchemaIncludesAPIContractPolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.schema.json"))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	healingRequest := properties["healingRequest"].(map[string]interface{})
	healingProperties := healingRequest["properties"].(map[string]interface{})
	apiContractPolicy, ok := healingProperties["apiContractPolicy"].(map[string]interface{})
	if !ok {
		t.Fatalf("apiContractPolicy missing in healingRequest properties")
	}
	required := apiContractPolicy["required"].([]interface{})
	if len(required) != 5 {
		t.Fatalf("unexpected apiContractPolicy required fields: %#v", required)
	}
}

func TestValuesSchemaIncludesReleaseReadinessPolicy(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.schema.json"))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	healingRequest := properties["healingRequest"].(map[string]interface{})
	healingProperties := healingRequest["properties"].(map[string]interface{})
	releasePolicy, ok := healingProperties["releaseReadinessPolicy"].(map[string]interface{})
	if !ok {
		t.Fatalf("releaseReadinessPolicy missing in healingRequest properties")
	}
	required := releasePolicy["required"].([]interface{})
	if len(required) != 5 {
		t.Fatalf("unexpected releaseReadinessPolicy required fields: %#v", required)
	}
	allowedDecisions := releasePolicy["properties"].(map[string]interface{})["allowedDecisions"].(map[string]interface{})
	if allowedDecisions["type"].(string) != "array" {
		t.Fatalf("allowedDecisions should be array")
	}
}

func TestValuesYamlIncludesReleaseReadinessPolicyDefaults(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.yaml"))
	if err != nil {
		t.Fatalf("read values yaml failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"releaseReadinessPolicy:",
		"enabled: true",
		"maxOpenIncidents: 3",
		"requireRollbackCandidate: true",
		"drillEvidenceTTLMinutes: 60",
		"- allow",
		"- degrade",
		"- block",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("values.yaml missing %s", token)
		}
	}
}

func TestValuesYamlIncludesAPIContractPolicyDefaults(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.yaml"))
	if err != nil {
		t.Fatalf("read values yaml failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{"apiContractPolicy:", "compatibilityClass: backward-compatible", "riskLevel: low", "requireStatusFields: true"} {
		if !strings.Contains(content, token) {
			t.Fatalf("values.yaml missing %s", token)
		}
	}
}

func TestValuesSchemaIncludesWorkloadKindEnum(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.schema.json"))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	healingRequest := properties["healingRequest"].(map[string]interface{})
	healingProperties := healingRequest["properties"].(map[string]interface{})
	workloadKind := healingProperties["workloadKind"].(map[string]interface{})
	enumValues := workloadKind["enum"].([]interface{})
	if len(enumValues) != 2 || enumValues[0].(string) != "Deployment" || enumValues[1].(string) != "StatefulSet" {
		t.Fatalf("unexpected workloadKind enum: %#v", enumValues)
	}
}

func TestValuesYamlDefaultsToDeploymentWorkloadKind(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.yaml"))
	if err != nil {
		t.Fatalf("read values yaml failed: %v", err)
	}
	content := string(raw)
	if !strings.Contains(content, "workloadKind: Deployment") {
		t.Fatalf("values.yaml should default workloadKind to Deployment for safe l1 mvp")
	}
}

func TestValuesSchemaIncludesDeliveryPipeline(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.schema.json"))
	if err != nil {
		t.Fatalf("read schema failed: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema failed: %v", err)
	}
	properties := schema["properties"].(map[string]interface{})
	healingRequest := properties["healingRequest"].(map[string]interface{})
	healingProperties := healingRequest["properties"].(map[string]interface{})
	deliveryPipeline, ok := healingProperties["deliveryPipeline"].(map[string]interface{})
	if !ok {
		t.Fatalf("deliveryPipeline missing in healingRequest properties")
	}
	required := deliveryPipeline["required"].([]interface{})
	if len(required) != 4 {
		t.Fatalf("unexpected deliveryPipeline required fields: %#v", required)
	}
}

func TestValuesYamlIncludesDeliveryPipelineDefaults(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("values.yaml"))
	if err != nil {
		t.Fatalf("read values yaml failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"deliveryPipeline:",
		"enabled: true",
		"nightlySchedule: '0 2 * * 1-5'",
		"drillWindow: weekdays",
		"evidenceRetentionDays: 14",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("values.yaml missing %s", token)
		}
	}
}
