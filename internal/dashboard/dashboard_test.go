package dashboard

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeCtl struct{ killed bool }

func (f *fakeCtl) State() State {
	return State{WindowLabel: "daily", BudgetUSD: 50, SpentUSD: 10, Killed: f.killed}
}
func (f *fakeCtl) Kill(string) { f.killed = true }

func TestDashboard(t *testing.T) {
	c := &fakeCtl{}
	srv := httptest.NewServer(Handler(c))
	defer srv.Close()

	// /api/state returns the controller's state as JSON.
	resp, err := http.Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	var st State
	_ = json.NewDecoder(resp.Body).Decode(&st)
	_ = resp.Body.Close()
	if st.BudgetUSD != 50 || st.WindowLabel != "daily" {
		t.Fatalf("state=%+v", st)
	}

	// / serves the embedded index page.
	ir, _ := http.Get(srv.URL + "/")
	b, _ := io.ReadAll(ir.Body)
	_ = ir.Body.Close()
	if !strings.Contains(string(b), "breaker") {
		t.Fatal("index page not served")
	}

	// POST /kill trips the controller.
	kreq, _ := http.NewRequest(http.MethodPost, srv.URL+"/kill", nil)
	kr, err := http.DefaultClient.Do(kreq)
	if err != nil {
		t.Fatal(err)
	}
	_ = kr.Body.Close()
	if kr.StatusCode != http.StatusNoContent || !c.killed {
		t.Fatalf("kill: status=%d killed=%v", kr.StatusCode, c.killed)
	}

	// GET /kill is rejected.
	gr, _ := http.Get(srv.URL + "/kill")
	_ = gr.Body.Close()
	if gr.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("GET /kill status=%d want 405", gr.StatusCode)
	}
}
