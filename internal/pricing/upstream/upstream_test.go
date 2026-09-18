package upstream

import (
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/anthropic-pricing.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return b
}

func TestParseAnthropic(t *testing.T) {
	rows, err := ParseAnthropic(fixture(t))
	if err != nil {
		t.Fatalf("ParseAnthropic: %v", err)
	}

	byID := map[string]Row{}
	for _, r := range rows {
		if _, dup := byID[r.Model]; dup {
			t.Errorf("duplicate row for %q", r.Model)
		}
		byID[r.Model] = r
	}

	want := map[string]Row{
		"claude-fable-5-1":  {Input: 10, CacheWrite: 12.5, CacheRead: 0.25, Output: 50},
		"claude-mythos-5-1": {Input: 10, CacheWrite: 12.5, CacheRead: 0.25, Output: 50},
		"claude-fable-5":    {Input: 10, CacheWrite: 12.5, CacheRead: 1, Output: 50},
		"claude-opus-5":     {Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25},
		"claude-opus-4-8":   {Input: 5, CacheWrite: 6.25, CacheRead: 0.5, Output: 25},
		"claude-sonnet-5":   {Input: 2, CacheWrite: 2.5, CacheRead: 0.2, Output: 10},
		"claude-sonnet-4-6": {Input: 3, CacheWrite: 3.75, CacheRead: 0.3, Output: 15},
		"claude-haiku-4-5":  {Input: 1, CacheWrite: 1.25, CacheRead: 0.1, Output: 5},
	}
	for id, w := range want {
		got, ok := byID[id]
		if !ok {
			t.Errorf("%s: missing from parsed rows", id)
			continue
		}
		if got.Input != w.Input || got.CacheWrite != w.CacheWrite ||
			got.CacheRead != w.CacheRead || got.Output != w.Output {
			t.Errorf("%s = in %v, write %v, read %v, out %v; want in %v, write %v, read %v, out %v",
				id, got.Input, got.CacheWrite, got.CacheRead, got.Output,
				w.Input, w.CacheWrite, w.CacheRead, w.Output)
		}
	}

	// Retired models are skipped: they are not on the first-party API, and the
	// table's conservative fallback covers anything unlisted.
	for _, id := range []string{"claude-opus-4-1", "claude-haiku-3-5"} {
		if _, ok := byID[id]; ok {
			t.Errorf("%s is marked retired on the page and should have been skipped", id)
		}
	}

	// The batch-pricing table further down the page must not be mistaken for the
	// model table; Opus 5 would read $2.50 in there.
	if r := byID["claude-opus-5"]; r.Input == 2.5 {
		t.Error("parsed the batch-pricing table instead of the model table")
	}
}

// TestParseAnthropicRefusesBadPages is the point of the exercise: a page whose
// shape we no longer recognise must produce an error, never a partial table
// that would quietly mis-price a model.
func TestParseAnthropicRefusesBadPages(t *testing.T) {
	full := string(fixture(t))
	cases := []struct {
		name string
		page string
	}{
		{"empty", ""},
		{"prose only", "# Pricing\n\nAll prices are in USD.\n"},
		{"table but no cache column", strings.Replace(full, "Cache hits and refreshes", "Cache stuff", 1)},
		{"table but no input column", strings.Replace(full, "Base input tokens", "Cost per token", 1)},
		{"header present, rows gone", headerOnly(full)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, err := ParseAnthropic([]byte(c.page))
			if err == nil {
				t.Fatalf("want an error, got %d rows", len(rows))
			}
		})
	}
}

// headerOnly keeps the table header and separator but drops every data row.
func headerOnly(page string) string {
	var out []string
	for _, l := range strings.Split(page, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "| Claude") {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

func TestPriceCell(t *testing.T) {
	ok := map[string]float64{
		"$10 / MTok":               10,
		"$12.50 / MTok":            12.5,
		"$0.25 / MTok<sup>1</sup>": 0.25,
		"  $1,250 / MTok  ":        1250,
		"$0.08 / MTok":             0.08,
	}
	for cell, want := range ok {
		got, err := price(cell)
		if err != nil || got != want {
			t.Errorf("price(%q) = %v, %v; want %v, nil", cell, got, err, want)
		}
	}
	for _, cell := range []string{"", "  ", "free", "$0 / MTok", "$-5 / MTok", "N/A"} {
		if got, err := price(cell); err == nil {
			t.Errorf("price(%q) = %v, nil; want an error", cell, got)
		}
	}
}

func TestModelID(t *testing.T) {
	cases := map[string]string{
		"Claude Opus 5":     "claude-opus-5",
		"Claude Fable 5.1":  "claude-fable-5-1",
		"Claude Sonnet 4.6": "claude-sonnet-4-6",
		"Claude Haiku 4.5":  "claude-haiku-4-5",
	}
	for display, want := range cases {
		if got := modelID(display); got != want {
			t.Errorf("modelID(%q) = %q; want %q", display, got, want)
		}
	}
}
