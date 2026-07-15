package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestServeRollingBudgetRefuses proves serve refuses requests with 402 once the
// rolling-window budget is crossed, and that the manual KILL endpoint works.
func TestServeRollingBudgetRefuses(t *testing.T) {
	dir := t.TempDir()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	build := func(pkg, name string) string {
		bin := filepath.Join(dir, name)
		c := exec.Command("go", "build", "-o", bin, pkg)
		c.Dir = root
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", pkg, err, out)
		}
		return bin
	}
	breakerBin := build("./cmd/breaker", "breaker")
	mockBin := build("./test/mockllm", "mockllm")

	mockPort, servePort := freePort(t), freePort(t)
	mock := exec.Command(mockBin, fmt.Sprintf("127.0.0.1:%d", mockPort))
	if err := mock.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = mock.Process.Kill() }()

	srv := exec.Command(breakerBin, "serve", "--daily", "0.05",
		"--port", fmt.Sprint(servePort),
		"--anthropic-upstream", fmt.Sprintf("http://127.0.0.1:%d", mockPort))
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Process.Kill() }()

	base := fmt.Sprintf("http://127.0.0.1:%d", servePort)
	waitReady(t, base+"/api/state")

	// Each call meters ≈ $0.018; the 4th must be refused once spend ≥ $0.05.
	want := []int{200, 200, 200, 402}
	for i, w := range want {
		code := post(t, base+"/v1/messages", `{"model":"claude-sonnet-5","stream":true}`)
		if code != w {
			t.Fatalf("request %d: HTTP %d; want %d", i+1, code, w)
		}
	}

	// State reflects the spend.
	var st struct {
		SpentUSD  float64 `json:"spent_usd"`
		BudgetUSD float64 `json:"budget_usd"`
	}
	resp, err := http.Get(base + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.NewDecoder(resp.Body).Decode(&st)
	resp.Body.Close()
	if st.SpentUSD < st.BudgetUSD {
		t.Fatalf("state spent %.4f < budget %.4f", st.SpentUSD, st.BudgetUSD)
	}

	// Manual KILL returns 204.
	kreq, _ := http.NewRequest(http.MethodPost, base+"/kill", nil)
	kresp, err := http.DefaultClient.Do(kreq)
	if err != nil {
		t.Fatal(err)
	}
	kresp.Body.Close()
	if kresp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /kill: HTTP %d; want 204", kresp.StatusCode)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitReady(t *testing.T, url string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("serve did not become ready")
}

func post(t *testing.T, url, body string) int {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}
