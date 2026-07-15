package pricing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
)

func TestLoadOverride(t *testing.T) {
	f := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(f, []byte(`{"version":"custom","models":{"my-model":{"input":1,"output":2}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tbl, err := Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if tbl.Version != "custom" {
		t.Fatalf("version=%q", tbl.Version)
	}
	got, matched := tbl.Cost("my-model", core.Usage{InputTokens: 1_000_000, OutputTokens: 1_000_000})
	if !matched || got != 3 {
		t.Fatalf("cost=%v matched=%v want 3,true", got, matched)
	}
	// Embedded models survive the merge.
	if _, m := tbl.Cost("claude-opus-4-8", core.Usage{InputTokens: 1_000_000}); !m {
		t.Fatal("embedded model lost after override merge")
	}
}

func TestLoadOverrideMissingFile(t *testing.T) {
	if _, err := Load("/no/such/breaker-prices.json"); err == nil {
		t.Fatal("expected an error for a missing override file")
	}
}
