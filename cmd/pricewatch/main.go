// Command pricewatch keeps internal/pricing/prices.json in step with the
// vendor's published prices.
//
// It only wires three things together: internal/pricing/upstream fetches and
// parses the page, (*pricing.Table).Reconcile decides what may be applied
// unattended, and this file writes the result and reports it. Every judgement
// about safety lives in Reconcile, so it can be tested without a network.
//
// Exit codes, which the scheduled workflow branches on:
//
//	0  the table already matches the published prices
//	3  the table changed (written when -write is given)
//	2  refused: the page or the table needs a human
//	1  the run itself failed (network, unreadable file)
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gmaOCR/breaker/internal/pricing"
	"github.com/gmaOCR/breaker/internal/pricing/upstream"
)

const (
	exitCurrent = 0
	exitFailed  = 1
	exitRefused = 2
	exitChanged = 3
)

func main() { os.Exit(run()) }

func run() int {
	file := flag.String("file", "internal/pricing/prices.json", "price table to read, and to rewrite with -write")
	src := flag.String("url", upstream.AnthropicURL, "pricing page URL, or a local path for offline runs")
	write := flag.Bool("write", false, "apply the changes to -file instead of only reporting them")
	timeout := flag.Duration("timeout", 30*time.Second, "deadline for fetching the pricing page")
	flag.Parse()

	cur, err := pricing.LoadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pricewatch: %v\n", err)
		return exitFailed
	}

	page, err := readPage(*src, *timeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pricewatch: %v\n", err)
		return exitFailed
	}

	rows, err := upstream.ParseAnthropic(page)
	if err != nil {
		// A page we cannot parse is not an outage: the layout changed and the
		// parser needs attention, which is a person's job.
		fmt.Fprintf(os.Stderr, "pricewatch: %v\n", err)
		return exitRefused
	}

	plan, err := cur.Reconcile(rows, time.Now().UTC().Format("2006-01-02"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "pricewatch: %v\n", err)
		return exitRefused
	}

	for _, s := range plan.Skipped {
		fmt.Fprintf(os.Stderr, "pricewatch: skipped %s\n", s)
	}

	if len(plan.Changes) == 0 {
		fmt.Printf("pricewatch: %d models checked, table %s is current\n", len(rows), cur.Version)
		return exitCurrent
	}

	fmt.Printf("pricewatch: %d change(s) against the published prices, table %s to %s\n",
		len(plan.Changes), cur.Version, plan.Table.Version)
	for _, c := range plan.Changes {
		fmt.Println("  " + c.String())
	}

	if !*write {
		return exitChanged
	}
	if err := plan.Table.WriteFile(*file); err != nil {
		fmt.Fprintf(os.Stderr, "pricewatch: %v\n", err)
		return exitFailed
	}
	fmt.Printf("pricewatch: wrote %s\n", *file)
	return exitChanged
}

// readPage takes either a URL or a local path, so the same binary can be run
// against a saved page with no network.
func readPage(src string, timeout time.Duration) ([]byte, error) {
	if _, err := os.Stat(src); err == nil {
		return os.ReadFile(src)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return upstream.Fetch(ctx, src)
}
