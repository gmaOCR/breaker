package pricing

import (
	"os"
	"strings"
	"testing"

	"github.com/gmaOCR/breaker/internal/core"
	"github.com/gmaOCR/breaker/internal/pricing/upstream"
)

// publishedRows parses the saved copy of the vendor pricing page that
// internal/pricing/upstream tests against, so these tests stay offline.
func publishedRows(t *testing.T) []upstream.Row {
	t.Helper()
	page, err := os.ReadFile("upstream/testdata/anthropic-pricing.md")
	if err != nil {
		t.Fatalf("read page: %v", err)
	}
	rows, err := upstream.ParseAnthropic(page)
	if err != nil {
		t.Fatalf("parse page: %v", err)
	}
	return rows
}

func rowFor(rows []upstream.Row, model string) upstream.Row {
	for _, r := range rows {
		if r.Model == model {
			return r
		}
	}
	return upstream.Row{}
}

func replace(rows []upstream.Row, r upstream.Row) []upstream.Row {
	out := append([]upstream.Row(nil), rows...)
	for i := range out {
		if out[i].Model == r.Model {
			out[i] = r
		}
	}
	return out
}

func drop(rows []upstream.Row, model string) []upstream.Row {
	var out []upstream.Row
	for _, r := range rows {
		if r.Model != model {
			out = append(out, r)
		}
	}
	return out
}

// TestEveryPublishedModelHasAKey is the regression test for the bug that started
// all this: a model priced at the high fallback because no pattern covered it.
//
// It deliberately asserts structure, not price levels. Whether the table is
// current is the scheduled pricewatch job's question, and it answers it against
// the live page; if this test demanded matching prices it would fail the moment
// a real price moved and would block the very job meant to fix it.
func TestEveryPublishedModelHasAKey(t *testing.T) {
	tbl, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rows := publishedRows(t)

	for _, r := range rows {
		if _, matched := tbl.Cost(r.Model, core.Usage{InputTokens: 1000}); !matched {
			t.Errorf("%s (%s) falls through to the fallback price; add a pattern for it", r.Model, r.Display)
		}
	}

	// Reconcile must also be willing to run unattended against this page: no key
	// left uncovered, no pattern gone ambiguous, no row failing its integrity
	// check. Any of those means the automation would refuse and page a human.
	plan, err := tbl.Reconcile(rows, "2026-09-18")
	if err != nil {
		t.Fatalf("Reconcile refuses the saved page, so the scheduled job would too: %v", err)
	}
	if len(plan.Skipped) != 0 {
		t.Errorf("rows skipped by an integrity check: %v", plan.Skipped)
	}
}

// TestReconcileStampsTheDate checks that a plan carrying changes also carries
// the supplied date, since that is what makes staleness visible.
func TestReconcileStampsTheDate(t *testing.T) {
	tbl, _ := Load("")
	rows := publishedRows(t)
	moved := rowFor(rows, "claude-haiku-4-5")
	moved.Input, moved.Output = 2, 10
	moved.CacheWrite, moved.CacheRead = 2.5, 0.2

	plan, err := tbl.Reconcile(replace(rows, moved), "2026-12-25")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(plan.Changes) == 0 {
		t.Fatal("expected a change")
	}
	if plan.Table.Version != "2026-12-25" {
		t.Errorf("version = %q; want the supplied date", plan.Table.Version)
	}
}

// TestReconcileIsStable applies a plan and reconciles again against the same
// page: the second pass must find nothing left to do, or the scheduled job would
// rewrite the table and cut a release on every run.
func TestReconcileIsStable(t *testing.T) {
	tbl, _ := Load("")
	rows := publishedRows(t)

	// Move a price so the first pass has something to apply.
	moved := rowFor(rows, "claude-sonnet-5")
	moved.Input, moved.Output = 4, 20
	moved.CacheWrite, moved.CacheRead = 5, 0.4
	rows = replace(rows, moved)

	first, err := tbl.Reconcile(rows, "2026-09-18")
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if len(first.Changes) == 0 {
		t.Fatal("first pass found nothing to apply; the fixture no longer drifts")
	}

	second, err := first.Table.Reconcile(rows, "2026-09-19")
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if len(second.Changes) != 0 {
		t.Errorf("second pass proposed %d changes, want none:\n%s", len(second.Changes), renderChanges(second))
	}
	if second.Table != nil {
		t.Error("a plan with no changes must leave Table nil")
	}
}

func TestReconcileDetectsDrift(t *testing.T) {
	tbl, _ := Load("")
	rows := publishedRows(t)

	// Sonnet 5 halves, multipliers kept consistent so integrity still holds.
	cheap := rowFor(rows, "claude-sonnet-5")
	cheap.Input, cheap.Output = 1, 5
	cheap.CacheWrite, cheap.CacheRead = 1.25, 0.1

	plan, err := tbl.Reconcile(replace(rows, cheap), "2026-09-18")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	var found *Change
	for i := range plan.Changes {
		if plan.Changes[i].Key == "claude-sonnet-5*" {
			found = &plan.Changes[i]
		}
	}
	if found == nil {
		t.Fatalf("no change for claude-sonnet-5*:\n%s", renderChanges(plan))
	}
	if found.Added || found.Before.Input != 2 || found.After.Input != 1 {
		t.Errorf("change = %v; want an update from 2 to 1", *found)
	}
}

// TestReconcileRefusesWhenAPricedRowFailsIntegrity: a row whose cache columns
// do not match the documented multipliers was mis-parsed. It is dropped, which
// leaves its key uncovered, which refuses the whole plan. Failing closed on the
// entire update is the point: we cannot trust one column of a page and not the
// rest of it.
func TestReconcileRefusesWhenAPricedRowFailsIntegrity(t *testing.T) {
	tbl, _ := Load("")
	rows := publishedRows(t)

	bad := rowFor(rows, "claude-opus-5")
	bad.Input = 0.5 // as if the column had shifted; cache columns now disagree

	if _, err := tbl.Reconcile(replace(rows, bad), "2026-09-18"); err == nil {
		t.Fatal("want an error when a priced model's row fails its integrity check")
	}
}

// TestReconcileSkipsUnknownRowsFailingIntegrity: the same failure on a model no
// pattern covers degrades gracefully instead. It is reported and left out, and
// the rest of the update still goes through, because that model keeps being
// priced by the conservative fallback either way.
func TestReconcileSkipsUnknownRowsFailingIntegrity(t *testing.T) {
	tbl, _ := Load("")

	// A model the table has no key for, with cache columns nothing like the
	// documented 1.25x. Fabricated rather than taken from the fixture: the
	// watcher adds new models to the table, so no real model stays unknown.
	bogus := upstream.Row{
		Model: "claude-nonesuch-9", Display: "Claude Nonesuch 9",
		Input: 10, Output: 50, CacheWrite: 90, CacheRead: 1,
	}
	rows := append(publishedRows(t), bogus)

	plan, err := tbl.Reconcile(rows, "2026-09-18")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(plan.Skipped) != 1 || !strings.Contains(plan.Skipped[0], "claude-nonesuch-9") {
		t.Fatalf("Skipped = %v; want exactly the fabricated row", plan.Skipped)
	}
	for _, c := range plan.Changes {
		if c.Key == "claude-nonesuch-9" {
			t.Errorf("wrote a price for a row that failed integrity: %v", c)
		}
	}
}

// TestReconcileRefusesWhenAPricedModelVanishes: a key that matches nothing on
// the page means the page changed or the model was delisted. Keeping the stale
// price silently is wrong and deleting the entry is worse, so stop.
func TestReconcileRefusesWhenAPricedModelVanishes(t *testing.T) {
	tbl, _ := Load("")
	rows := drop(publishedRows(t), "claude-haiku-4-5")
	if _, err := tbl.Reconcile(rows, "2026-09-18"); err == nil {
		t.Fatal("want an error when claude-haiku-4-5* matches no published model")
	}
}

// TestReconcileRefusesAmbiguousPattern: one glob covering two models that are no
// longer priced alike cannot be resolved without splitting the pattern.
func TestReconcileRefusesAmbiguousPattern(t *testing.T) {
	tbl, _ := Load("")
	rows := publishedRows(t)

	split := rowFor(rows, "claude-opus-4-5") // shares claude-opus-4-[5-8]* with 4-8
	split.Input, split.Output = 8, 40
	split.CacheWrite, split.CacheRead = 10, 0.8

	_, err := tbl.Reconcile(replace(rows, split), "2026-09-18")
	if err == nil {
		t.Fatal("want an error when one pattern covers models priced differently")
	}
	if !strings.Contains(err.Error(), "claude-opus-4-[5-8]*") {
		t.Errorf("error should name the ambiguous key, got: %v", err)
	}
}

// TestReconcileLeavesNonClaudeEntriesAlone: OpenAI prices are context-tiered and
// this table cannot express that, so they stay on whatever is shipped.
func TestReconcileLeavesNonClaudeEntriesAlone(t *testing.T) {
	tbl, _ := Load("")
	before := tbl.Models["gpt-4o"]

	// Drift a Claude price so the plan actually produces a table to inspect.
	rows := publishedRows(t)
	moved := rowFor(rows, "claude-opus-5")
	moved.Input, moved.Output = 6, 30
	moved.CacheWrite, moved.CacheRead = 7.5, 0.6

	plan, err := tbl.Reconcile(replace(rows, moved), "2026-09-18")
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if after := plan.Table.Models["gpt-4o"]; after != before {
		t.Errorf("gpt-4o changed from %+v to %+v", before, after)
	}
	if plan.Table.Fallback != tbl.Fallback {
		t.Error("the fallback must never be rewritten unattended")
	}
}

// TestMatchKeyMirrorsLookup is the invariant the whole scheme rests on: if
// Reconcile writes to a key the proxy would not read for that model, a price
// update lands in the wrong place and nothing reports it.
func TestMatchKeyMirrorsLookup(t *testing.T) {
	tbl, _ := Load("")
	models := []string{
		"claude-opus-5", "claude-opus-4-8", "claude-opus-4-5-20251101",
		"claude-sonnet-5", "claude-sonnet-4-6", "claude-haiku-4-5",
		"claude-fable-5", "claude-fable-5-1", "gpt-4o", "gpt-4.1-mini",
	}
	oneM := core.Usage{InputTokens: 1_000_000, CacheReadTokens: 1_000_000}
	for _, m := range models {
		key, ok := tbl.matchKey(m)
		if !ok {
			t.Errorf("%s: matchKey found nothing but lookup would", m)
			continue
		}
		viaKey := tbl.Models[key]
		want := viaKey.Input + viaKey.CacheRead
		got, matched := tbl.Cost(m, oneM)
		if !matched {
			t.Errorf("%s: Cost reports unmatched but matchKey returned %q", m, key)
			continue
		}
		if got != want {
			t.Errorf("%s: Cost = %v but key %q prices it at %v", m, got, key, want)
		}
	}
}

func renderChanges(p *Plan) string {
	var b strings.Builder
	for _, c := range p.Changes {
		b.WriteString("  " + c.String() + "\n")
	}
	return b.String()
}
