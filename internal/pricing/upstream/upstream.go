// Package upstream turns a vendor pricing page into plain rows. It knows
// nothing about breaker's price table, about files, or about what may be
// changed: fetching and parsing are kept apart from policy so the parser can be
// tested offline against a saved copy of the page, and so the rules about what
// is safe to apply live in one place (pricing.Reconcile) rather than here.
package upstream

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// AnthropicURL serves the pricing page as raw Markdown, which is why this
// package can parse it with the standard library instead of scraping HTML.
const AnthropicURL = "https://platform.claude.com/docs/en/about-claude/pricing.md"

// Row is one model's published prices, in USD per million tokens.
type Row struct {
	Model      string  // derived id, e.g. "claude-opus-5"
	Display    string  // as printed on the page, e.g. "Claude Opus 5"
	Input      float64 // base input
	CacheWrite float64 // 5-minute cache write
	CacheRead  float64 // cache hit or refresh
	Output     float64
}

// Fetch retrieves a pricing page. The caller supplies the URL so tests and the
// CLI can point at a local copy.
func Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("upstream: build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream: fetch %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream: fetch %s: HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("upstream: read %s: %w", url, err)
	}
	return body, nil
}

var (
	linkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	supRe  = regexp.MustCompile(`<sup>.*?</sup>`)
)

// ParseAnthropic extracts the model pricing table. Columns are located by their
// header text, not by position, so the page may gain or reorder columns without
// silently shifting prices into the wrong field. Retired models are skipped:
// they are not served on the first-party API this proxy fronts, and the table's
// conservative fallback already covers anything unlisted.
func ParseAnthropic(page []byte) ([]Row, error) {
	lines := strings.Split(string(page), "\n")

	hdr, cols := -1, map[string]int(nil)
	for i, l := range lines {
		if c := headerColumns(splitCells(l)); c != nil {
			hdr, cols = i, c
			break
		}
	}
	if hdr < 0 {
		return nil, errors.New("upstream: no model pricing table found; the page layout has changed")
	}

	var rows []Row
	for _, l := range lines[hdr+2:] { // +2 skips the |---|---| separator
		cells := splitCells(l)
		if len(cells) == 0 {
			break // end of the table
		}
		if len(cells) <= cols["output"] {
			continue
		}
		display, retired := modelName(cells[cols["model"]])
		if display == "" || retired {
			continue
		}
		r := Row{Model: modelID(display), Display: display}
		var err error
		for field, dst := range map[string]*float64{
			"input": &r.Input, "write": &r.CacheWrite,
			"read": &r.CacheRead, "output": &r.Output,
		} {
			if *dst, err = price(cells[cols[field]]); err != nil {
				return nil, fmt.Errorf("upstream: %s, %s column: %w", display, field, err)
			}
		}
		rows = append(rows, r)
	}
	if len(rows) == 0 {
		return nil, errors.New("upstream: pricing table found but no rows parsed")
	}
	return rows, nil
}

// headerColumns returns the column index of each field we need, or nil when
// these cells are not the pricing table's header. Every field must be present:
// a partial match means the page changed and we would rather stop than guess.
func headerColumns(cells []string) map[string]int {
	want := map[string]string{
		"model":  "model",
		"input":  "base input",
		"write":  "5m cache write",
		"read":   "cache hits",
		"output": "output token",
	}
	got := map[string]int{}
	for i, c := range cells {
		lc := strings.ToLower(c)
		for field, needle := range want {
			if _, done := got[field]; !done && strings.Contains(lc, needle) {
				got[field] = i
			}
		}
	}
	if len(got) != len(want) {
		return nil
	}
	return got
}

// splitCells splits a Markdown table row, or returns nil for anything else.
func splitCells(line string) []string {
	l := strings.TrimSpace(line)
	if !strings.HasPrefix(l, "|") {
		return nil
	}
	parts := strings.Split(strings.Trim(l, "|"), "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

// modelName unwraps a model cell to its plain display name, reporting whether
// the page marks it retired.
func modelName(cell string) (name string, retired bool) {
	c := linkRe.ReplaceAllString(cell, "$1")
	c = supRe.ReplaceAllString(c, "")
	note := ""
	if i := strings.Index(c, "("); i >= 0 {
		note, c = strings.ToLower(c[i:]), c[:i]
	}
	c = strings.TrimSpace(c)
	if !strings.HasPrefix(strings.ToLower(c), "claude ") {
		return "", false
	}
	return c, strings.Contains(note, "retired")
}

// modelID derives the API model id from the display name, following the
// vendor's own convention: lowercase, and both spaces and dots become hyphens
// ("Claude Fable 5.1" is served as "claude-fable-5-1").
func modelID(display string) string {
	id := strings.ToLower(display)
	id = strings.ReplaceAll(id, " ", "-")
	return strings.ReplaceAll(id, ".", "-")
}

// price reads a "$12.50 / MTok" cell.
func price(cell string) (float64, error) {
	c := supRe.ReplaceAllString(cell, "")
	if i := strings.Index(c, "/"); i >= 0 {
		c = c[:i]
	}
	c = strings.TrimSpace(strings.NewReplacer("$", "", ",", "", "*", "").Replace(c))
	if c == "" {
		return 0, errors.New("empty price cell")
	}
	v, err := strconv.ParseFloat(c, 64)
	if err != nil {
		return 0, fmt.Errorf("unparseable price %q", cell)
	}
	if v <= 0 {
		return 0, fmt.Errorf("non-positive price %q", cell)
	}
	return v, nil
}
