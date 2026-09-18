package pricing

import (
	"fmt"
	"math"
	"path"
	"sort"
	"strings"

	"github.com/gmaOCR/breaker/internal/pricing/upstream"
)

// This file holds the policy for accepting published prices without a human in
// the loop. Parsing lives in internal/pricing/upstream; applying and committing
// live in cmd/pricewatch. The rules here exist because of one asymmetry: pricing
// a model too high only trips the breaker early, while pricing it too low hands
// the user the bill they installed breaker to avoid. So every rule below fails
// closed.

// centTolerance is half a cent per million tokens. Published prices carry two
// decimals, so anything beyond this is a parse error, not rounding.
const centTolerance = 0.005

// Cache multipliers Anthropic documents against the base input price. They are
// the integrity check that makes unattended updates safe: the input column is
// verified against two other independently parsed columns, so a shifted or
// mis-read column cannot slip through looking plausible.
const (
	cacheWriteMultiplier = 1.25
	cacheReadMultiplier  = 0.10
	cacheReadReduced     = 0.025 // Fable 5.1 and Mythos 5.1
)

// Change is one edit Reconcile proposes to the table.
type Change struct {
	Key    string // table key being written: an existing pattern, or a new exact id
	Model  string // the upstream model id that justifies the edit
	Added  bool   // true when Key did not exist before
	Before modelPrice
	After  modelPrice
}

// String renders a change as one reviewable line.
func (c Change) String() string {
	verb := "update"
	if c.Added {
		verb = "add   "
	}
	if c.Added {
		return fmt.Sprintf("%s %-24s in %.4g out %.4g write %.4g read %.4g",
			verb, c.Key, c.After.Input, c.After.Output, c.After.CacheWrite, c.After.CacheRead)
	}
	return fmt.Sprintf("%s %-24s in %.4g>%.4g out %.4g>%.4g write %.4g>%.4g read %.4g>%.4g",
		verb, c.Key,
		c.Before.Input, c.After.Input, c.Before.Output, c.After.Output,
		c.Before.CacheWrite, c.After.CacheWrite, c.Before.CacheRead, c.After.CacheRead)
}

// Plan is the outcome of reconciling a table against published prices.
type Plan struct {
	Changes []Change // empty means the table is already current
	Skipped []string // rows a check rejected, with the reason, for reporting
	Table   *Table   // the table with Changes applied; nil when there is nothing to do
}

// Reconcile compares the table's Claude entries with published rows and returns
// the edits that may be applied unattended. It fails, rather than returning a
// partial plan, when the comparison itself looks untrustworthy: a model the
// table prices has vanished from the page, or one table pattern now covers
// models whose prices differ. Both mean a human has to look, so neither is
// something to resolve by guessing.
//
// Non-Claude entries and the fallback are never touched: OpenAI publishes
// context-tiered prices that this table cannot express, so leaving those models
// on the high fallback is deliberate.
func (t *Table) Reconcile(rows []upstream.Row, today string) (*Plan, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("pricing: no published rows to reconcile against")
	}

	plan := &Plan{}
	// Rows grouped by the table key they would resolve to, so a pattern covering
	// several models can be checked for agreement before anything is written.
	perKey := map[string][]upstream.Row{}
	var newKeys []upstream.Row

	for _, r := range rows {
		if err := checkIntegrity(r); err != nil {
			plan.Skipped = append(plan.Skipped, fmt.Sprintf("%s: %v", r.Model, err))
			continue
		}
		if key, ok := t.matchKey(r.Model); ok {
			perKey[key] = append(perKey[key], r)
			continue
		}
		newKeys = append(newKeys, r)
	}

	// Every Claude entry the table prices must still appear on the page. A key
	// that no longer matches anything means the page changed shape or the model
	// was delisted; either way, silently keeping a stale price is the wrong
	// answer and silently deleting an entry is worse.
	for key := range t.Models {
		if !isClaudeKey(key) {
			continue
		}
		if len(perKey[key]) == 0 {
			return nil, fmt.Errorf("pricing: table key %q matches no published model; refusing to update unattended", key)
		}
	}

	for _, key := range sortedKeys(perKey) {
		group := perKey[key]
		want := priceOf(group[0])
		for _, r := range group[1:] {
			if !samePrice(want, priceOf(r)) {
				return nil, fmt.Errorf("pricing: key %q covers %s and %s, which are now priced differently; split the pattern by hand",
					key, group[0].Model, r.Model)
			}
		}
		if before := t.Models[key]; !samePrice(before, want) {
			plan.Changes = append(plan.Changes, Change{Key: key, Model: group[0].Model, Before: before, After: want})
		}
	}

	// A model no pattern covers is added under its exact id rather than a new
	// glob: exact keys win over globs in lookup and cannot overlap each other,
	// so an unattended addition can never make an existing pattern ambiguous.
	sort.Slice(newKeys, func(i, j int) bool { return newKeys[i].Model < newKeys[j].Model })
	for _, r := range newKeys {
		plan.Changes = append(plan.Changes, Change{Key: r.Model, Model: r.Model, Added: true, After: priceOf(r)})
	}

	if len(plan.Changes) == 0 {
		return plan, nil
	}

	next := &Table{Version: today, Unit: t.Unit, Fallback: t.Fallback, Models: map[string]modelPrice{}}
	for k, v := range t.Models {
		next.Models[k] = v
	}
	for _, c := range plan.Changes {
		next.Models[c.Key] = c.After
	}
	plan.Table = next
	return plan, nil
}

// checkIntegrity verifies a row against the documented cache multipliers.
func checkIntegrity(r upstream.Row) error {
	if r.Input <= 0 || r.Output <= 0 {
		return fmt.Errorf("non-positive price (input %v, output %v)", r.Input, r.Output)
	}
	if want := r.Input * cacheWriteMultiplier; math.Abs(r.CacheWrite-want) > centTolerance {
		return fmt.Errorf("cache write %v is not %vx the %v input (expected %v)", r.CacheWrite, cacheWriteMultiplier, r.Input, want)
	}
	full, reduced := r.Input*cacheReadMultiplier, r.Input*cacheReadReduced
	if math.Abs(r.CacheRead-full) > centTolerance && math.Abs(r.CacheRead-reduced) > centTolerance {
		return fmt.Errorf("cache read %v is neither %vx nor %vx the %v input (expected %v or %v)",
			r.CacheRead, cacheReadMultiplier, cacheReadReduced, r.Input, full, reduced)
	}
	return nil
}

// matchKey reports which table key a model resolves to, mirroring lookup: an
// exact key wins, then the first matching glob.
func (t *Table) matchKey(model string) (string, bool) {
	if _, ok := t.Models[model]; ok {
		return model, true
	}
	for _, pat := range sortedKeys(t.Models) {
		if ok, _ := path.Match(pat, model); ok {
			return pat, true
		}
	}
	return "", false
}

func priceOf(r upstream.Row) modelPrice {
	return modelPrice{Input: r.Input, Output: r.Output, CacheWrite: r.CacheWrite, CacheRead: r.CacheRead}
}

func samePrice(a, b modelPrice) bool {
	return math.Abs(a.Input-b.Input) <= centTolerance &&
		math.Abs(a.Output-b.Output) <= centTolerance &&
		math.Abs(a.CacheWrite-b.CacheWrite) <= centTolerance &&
		math.Abs(a.CacheRead-b.CacheRead) <= centTolerance
}

func isClaudeKey(key string) bool { return strings.HasPrefix(key, "claude-") }

// sortedKeys keeps iteration deterministic; map order would otherwise make both
// glob matching and the reported diff vary between runs.
func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
