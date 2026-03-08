package scripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCRDIncludesConsolePrinterColumns(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "config", "crd", "_healingrequests.yaml"))
	if err != nil {
		t.Fatalf("read crd failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"additionalPrinterColumns:",
		"name: Phase",
		"jsonPath: .status.phase",
		"name: Action",
		"jsonPath: .status.lastAction",
		"name: Reason",
		"jsonPath: .status.blockReasonCode",
		"name: Recommendation",
		"jsonPath: .status.nextRecommendation",
		"name: Correlation",
		"jsonPath: .status.correlationKey",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("expected crd to contain %q", token)
		}
	}
}

func TestInstallManifestExposesMetricsScrapeService(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "config", "install", "kube-sentinel.yaml"))
	if err != nil {
		t.Fatalf("read install manifest failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"name: kube-sentinel-metrics",
		"app.kubernetes.io/component: metrics",
		"prometheus.io/scrape: 'true'",
		"prometheus.io/path: /metrics",
		"prometheus.io/port: '8080'",
		"port: 8080",
		"targetPort: metrics",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("expected install manifest to contain %q", token)
		}
	}
}

func TestServiceMonitorTargetsMetricsService(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "config", "monitoring", "kube-sentinel-servicemonitor.yaml"))
	if err != nil {
		t.Fatalf("read servicemonitor failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"kind: ServiceMonitor",
		"app.kubernetes.io/component: metrics",
		"port: metrics",
		"path: /metrics",
		"- kube-sentinel-system",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("expected servicemonitor to contain %q", token)
		}
	}
}

func TestGrafanaDashboardIncludesRequiredPanelGroups(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "config", "monitoring", "kube-sentinel-grafana-dashboard.json"))
	if err != nil {
		t.Fatalf("read dashboard failed: %v", err)
	}
	var dashboard map[string]interface{}
	if err := json.Unmarshal(raw, &dashboard); err != nil {
		t.Fatalf("unmarshal dashboard failed: %v", err)
	}
	content := string(raw)
	for _, token := range []string{
		"总体触发与成功率",
		"策略执行结果",
		"快照结果",
		"门禁与发布趋势",
		"kube_sentinel_triggers_total",
		"kube_sentinel_success_total",
		"kube_sentinel_deployment_l1_results_total",
		"kube_sentinel_snapshot_creates_total",
		"kube_sentinel_deployment_stage_blocks_total",
		"kube_sentinel_release_readiness_summary_total",
	} {
		if !strings.Contains(content, token) {
			t.Fatalf("expected dashboard to contain %q", token)
		}
	}
	if dashboard["title"].(string) == "" {
		t.Fatalf("dashboard title should not be empty")
	}
	panels, ok := dashboard["panels"].([]interface{})
	if !ok || len(panels) < 8 {
		t.Fatalf("expected dashboard panels to be present")
	}
}

func TestOpsConsoleDocsAreDiscoverableFromREADME(t *testing.T) {
	readmeRaw, err := os.ReadFile(filepath.Join("..", "README.md"))
	if err != nil {
		t.Fatalf("read README failed: %v", err)
	}
	docsRaw, err := os.ReadFile(filepath.Join("..", "docs", "ops-console.md"))
	if err != nil {
		t.Fatalf("read ops-console doc failed: %v", err)
	}
	readme := string(readmeRaw)
	docs := string(docsRaw)
	for _, token := range []string{
		"docs/ops-console.md",
		"kube-sentinel-metrics",
		"对象视图验证",
		"指标视图验证",
	} {
		if !strings.Contains(readme, token) {
			t.Fatalf("expected README to contain %q", token)
		}
	}
	for _, token := range []string{
		"Headlamp",
		"Grafana",
		"ServiceMonitor",
		"kube-sentinel-grafana-dashboard.json",
	} {
		if !strings.Contains(docs, token) {
			t.Fatalf("expected ops-console doc to contain %q", token)
		}
	}
}
