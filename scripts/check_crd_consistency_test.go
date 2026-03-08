package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runCRDConsistencyScript(t *testing.T, sourceDir, generatedDir string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", "./check-crd-consistency.sh")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(),
		"CRD_CHECK_SKIP_GENERATE=1",
		"CRD_SOURCE_DIR="+sourceDir,
		"CRD_GENERATED_DIR="+generatedDir,
	)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func TestCheckCRDConsistencyPassesWhenNoDrift(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	generatedDir := filepath.Join(tempDir, "generated")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source failed: %v", err)
	}
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatalf("mkdir generated failed: %v", err)
	}
	content := []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: healingrequests.kubesentinel.io\nspec:\n  group: kubesentinel.io\n  versions:\n  - name: v1alpha1\n")
	if err := os.WriteFile(filepath.Join(sourceDir, "_healingrequests.yaml"), content, 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "_healingrequests.yaml"), content, 0o644); err != nil {
		t.Fatalf("write generated file failed: %v", err)
	}

	output, err := runCRDConsistencyScript(t, sourceDir, generatedDir)
	if err != nil {
		t.Fatalf("expected pass, got error: %v output: %s", err, output)
	}
	if !strings.Contains(output, "QUALITY_GATE_RESULT=allow") {
		t.Fatalf("expected allow result, output: %s", output)
	}
}

func TestCheckCRDConsistencyPassesWhenVersionNameFollowsPrinterColumns(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	generatedDir := filepath.Join(tempDir, "generated")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source failed: %v", err)
	}
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatalf("mkdir generated failed: %v", err)
	}
	content := []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: healingrequests.kubesentinel.io\nspec:\n  group: kubesentinel.io\n  versions:\n    - additionalPrinterColumns:\n        - jsonPath: .status.phase\n          name: Phase\n          type: string\n      name: v1alpha1\n")
	if err := os.WriteFile(filepath.Join(sourceDir, "_healingrequests.yaml"), content, 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "_healingrequests.yaml"), content, 0o644); err != nil {
		t.Fatalf("write generated file failed: %v", err)
	}

	output, err := runCRDConsistencyScript(t, sourceDir, generatedDir)
	if err != nil {
		t.Fatalf("expected pass for version name after printer columns, got error: %v output: %s", err, output)
	}
	if !strings.Contains(output, "QUALITY_GATE_RESULT=allow") {
		t.Fatalf("expected allow result, output: %s", output)
	}
}

func TestCheckCRDConsistencyBlocksWhenDriftDetected(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	generatedDir := filepath.Join(tempDir, "generated")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source failed: %v", err)
	}
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatalf("mkdir generated failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "_healingrequests.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: healingrequests.kubesentinel.io\nspec:\n  group: kubesentinel.io\n  scope: Namespaced\n  versions:\n  - name: v1alpha1\n"), 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "_healingrequests.yaml"), []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: healingrequests.kubesentinel.io\nspec:\n  group: kubesentinel.io\n  scope: Cluster\n  versions:\n  - name: v1alpha1\n"), 0o644); err != nil {
		t.Fatalf("write generated file failed: %v", err)
	}

	output, err := runCRDConsistencyScript(t, sourceDir, generatedDir)
	if err == nil {
		t.Fatalf("expected drift to fail, output: %s", output)
	}
	for _, token := range []string{
		"QUALITY_GATE_RESULT=block",
		"QUALITY_GATE_CATEGORY=crd_consistency",
		"QUALITY_GATE_REASON=crd_generation_drift",
		"QUALITY_GATE_FIX_HINT=run:",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("missing token %s in output: %s", token, output)
		}
	}
}

func TestCheckCRDConsistencyBlocksWhenManifestInvalid(t *testing.T) {
	tempDir := t.TempDir()
	sourceDir := filepath.Join(tempDir, "source")
	generatedDir := filepath.Join(tempDir, "generated")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("mkdir source failed: %v", err)
	}
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatalf("mkdir generated failed: %v", err)
	}
	invalid := []byte("apiVersion: apiextensions.k8s.io/v1\nkind: CustomResourceDefinition\nmetadata:\n  name: healingrequests.\nspec:\n  group: \"\"\n  versions:\n  - name: \"\"\n")
	if err := os.WriteFile(filepath.Join(sourceDir, "_healingrequests.yaml"), invalid, 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "_healingrequests.yaml"), invalid, 0o644); err != nil {
		t.Fatalf("write generated file failed: %v", err)
	}

	output, err := runCRDConsistencyScript(t, sourceDir, generatedDir)
	if err == nil {
		t.Fatalf("expected invalid manifest to fail, output: %s", output)
	}
	for _, token := range []string{
		"QUALITY_GATE_RESULT=block",
		"QUALITY_GATE_CATEGORY=crd_consistency",
		"QUALITY_GATE_REASON=crd_manifest_invalid",
		"expected metadata.name=healingrequests.kubesentinel.io",
	} {
		if !strings.Contains(output, token) {
			t.Fatalf("missing token %s in output: %s", token, output)
		}
	}
}
