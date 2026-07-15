package e2e

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestBreakerKillsRunOverBudget is the headline proof: breaker run must SIGKILL
// a wrapped process once its metered LLM spend crosses the budget.
func TestBreakerKillsRunOverBudget(t *testing.T) {
	dir := t.TempDir()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	buildInto := func(pkg, name string) string {
		bin := filepath.Join(dir, name)
		c := exec.Command("go", "build", "-o", bin, pkg)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, out)
		}
		return bin
	}
	breakerBin := buildInto("./cmd/breaker", "breaker")
	mockBin := buildInto("./test/mockllm", "mockllm")
	loopBin := buildInto("./test/loopclient", "loopclient")

	// Start the fake upstream on a random port; read the chosen address.
	mock := exec.Command(mockBin, "127.0.0.1:0")
	stdout, err := mock.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mock.Process.Kill() }()
	mockAddr, _ := bufio.NewReader(stdout).ReadString('\n')
	mockAddr = strings.TrimSpace(mockAddr)
	if mockAddr == "" {
		t.Fatal("mock did not report its address")
	}

	// Budget $0.05; each call meters ≈ $0.018, so it trips on the 3rd call.
	cmd := exec.Command(breakerBin, "run", "--budget", "0.05", "--grace", "300ms",
		"--anthropic-upstream", "http://"+mockAddr, "--", loopBin)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		code := 0
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		if code != 137 {
			t.Fatalf("exit code = %d; want 137 (killed)\nstderr:\n%s", code, stderr.String())
		}
		if !strings.Contains(stderr.String(), "TRIPPED") {
			t.Fatalf("expected TRIPPED in breaker output\nstderr:\n%s", stderr.String())
		}
		t.Logf("breaker output:\n%s", stderr.String())
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("breaker did not kill the run within 15s\nstderr:\n%s", stderr.String())
	}
}
