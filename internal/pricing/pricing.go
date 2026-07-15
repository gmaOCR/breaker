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
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cache_write"`
	CacheRead  float64 `json:"cache_read"`
}

// Table is a dated set of per-model prices (USD per million tokens).
type Table struct {
	Version  string                `json:"version"`
	Unit     string                `json:"unit"`
	Models   map[string]modelPrice `json:"models"`
	Fallback modelPrice            `json:"fallback"`
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
// flag the result as estimated. Cost never returns zero for real usage — a zero
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
	// ponytail: glob match order is map-random; patterns are non-overlapping in
	// practice. Sort keys if that ever stops being true.
	for pat, p := range t.Models {
		if ok, _ := path.Match(pat, model); ok {
			return p, true
		}
	}
	return t.Fallback, false
}
