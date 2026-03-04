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
	content := []byte("kind: CustomResourceDefinition\nmetadata:\n  name: demo\n")
	if err := os.WriteFile(filepath.Join(sourceDir, "demo.yaml"), content, 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "demo.yaml"), content, 0o644); err != nil {
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
	if err := os.WriteFile(filepath.Join(sourceDir, "demo.yaml"), []byte("spec:\n  version: v1\n"), 0o644); err != nil {
		t.Fatalf("write source file failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "demo.yaml"), []byte("spec:\n  version: v2\n"), 0o644); err != nil {
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
