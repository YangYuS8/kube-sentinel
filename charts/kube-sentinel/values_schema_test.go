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
