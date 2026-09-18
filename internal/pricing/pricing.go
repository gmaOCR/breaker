// Package pricing turns token usage into USD using an embedded, overridable table.
package pricing

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path"

	"github.com/gmaOCR/breaker/internal/core"
)

//go:embed prices.json
var defaultTable []byte

type modelPrice struct {
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
	// Cache rates are omitted when zero so a provider that does not price cache
	// separately stays absent from the table rather than appearing to charge
	// nothing for it. omitempty affects writing only; reading is unchanged.
	CacheWrite float64 `json:"cache_write,omitempty"`
	CacheRead  float64 `json:"cache_read,omitempty"`
}

// Table is a dated set of per-model prices (USD per million tokens).
type Table struct {
	Version  string                `json:"version"`
	Unit     string                `json:"unit"`
	Models   map[string]modelPrice `json:"models"`
	Fallback modelPrice            `json:"fallback"`
}

// LoadFile parses a price table file on its own, with no embedded table
// underneath it. Load is for running breaker; this is for tools that read a
// table in order to rewrite it, where merging would hide what the file says.
func LoadFile(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("pricing: read %q: %w", path, err)
	}
	var t Table
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("pricing: parse %q: %w", path, err)
	}
	if len(t.Models) == 0 {
		return nil, fmt.Errorf("pricing: %q lists no models", path)
	}
	return &t, nil
}

// WriteFile renders the table back to disk, keys sorted by encoding/json so the
// output is stable and diffs stay readable.
func (t *Table) WriteFile(path string) error {
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("pricing: render table: %w", err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("pricing: write %q: %w", path, err)
	}
	return nil
}

// Load parses the embedded table, then shallow-merges an optional override file
// (per model). override may be "".
func Load(override string) (*Table, error) {
	var t Table
	if err := json.Unmarshal(defaultTable, &t); err != nil {
		return nil, fmt.Errorf("pricing: parse embedded table: %w", err)
	}
	if override == "" {
		return &t, nil
	}
	raw, err := os.ReadFile(override)
	if err != nil {
		return nil, fmt.Errorf("pricing: read override %q: %w", override, err)
	}
	var o Table
	if err := json.Unmarshal(raw, &o); err != nil {
		return nil, fmt.Errorf("pricing: parse override %q: %w", override, err)
	}
	if o.Version != "" {
		t.Version = o.Version
	}
	for name, p := range o.Models {
		t.Models[name] = p
	}
	if o.Fallback != (modelPrice{}) {
		t.Fallback = o.Fallback
	}
	return &t, nil
}

// Cost returns the USD cost of usage for model. The bool is false when no model
// pattern matched and the (deliberately high) fallback price was used, so callers
// flag the result as estimated. Cost never returns zero for real usage, a zero
// price would mean the breaker never trips, the one bug that kills the product.
func (t *Table) Cost(model string, u core.Usage) (float64, bool) {
	p, matched := t.lookup(model)
	const perMTok = 1_000_000.0
	cost := float64(u.InputTokens)*p.Input/perMTok +
		float64(u.OutputTokens)*p.Output/perMTok +
		float64(u.CacheWriteTokens)*p.CacheWrite/perMTok +
		float64(u.CacheReadTokens)*p.CacheRead/perMTok
	return cost, matched
}

func (t *Table) lookup(model string) (modelPrice, bool) {
	if p, ok := t.Models[model]; ok {
		return p, true
	}
	// Globs are tried in sorted order so the result is deterministic even if two
	// patterns ever overlap. Reconcile's matchKey mirrors this exactly; if the
	// two ever disagree, an unattended price update could write to a key the
	// proxy does not actually read.
	for _, pat := range sortedKeys(t.Models) {
		if ok, _ := path.Match(pat, model); ok {
			return t.Models[pat], true
		}
	}
	return t.Fallback, false
}
