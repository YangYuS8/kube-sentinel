package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func writeExecutable(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s failed: %v", name, err)
	}
	return path
}

func runShellScript(t *testing.T, script string, args []string, env map[string]string) (string, error) {
	t.Helper()
	commandArgs := append([]string{script}, args...)
	cmd := exec.Command("bash", commandArgs...)
	cmd.Dir = "."
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}
