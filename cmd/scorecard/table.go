package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/watarus/nansen/internal/pipeline"
	"github.com/watarus/nansen/internal/score"
)

// colour codes; disabled by NO_COLOR (https://no-color.org/) or a
// non-terminal stdout, per this project's own --dry-run/--offline
// verification runs (NO_COLOR=1).
const (
	ansiReset  = "\x1b[0m"
	ansiBold   = "\x1b[1m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiCyan   = "\x1b[36m"
	ansiGray   = "\x1b[90m"
)

func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func colorize(s, code string) string {
	if !colorEnabled() {
		return s
	}
	return code + s + ansiReset
}

func scoreColor(s float64) string {
	switch {
	case s >= 0.7:
		return ansiGreen
	case s >= 0.4:
		return ansiYellow
	default:
		return ansiRed
	}
}

// verdictColor per docs/DESIGN.md: BOTH/DISTRIBUTING/EXIT_ONLY are the
// falsifier's exit signal firing (red — "exactly as damning" whether or
// not independence is known), THIN is a caution (yellow), CONFIRMED
// survived both checks (green), WEAK is uninformative (gray).
func verdictColor(v score.Verdict) string {
	switch v {
	case score.VerdictBoth, score.VerdictDistributing, score.VerdictExitOnly:
		return ansiRed
	case score.VerdictThin:
		return ansiYellow
	case score.VerdictConfirmed:
		return ansiGreen
	default: // WEAK, CONCENTRATED
		return ansiGray
	}
}

// verdictLabel renders EXIT_ONLY as DISTRIBUTING: the verdict badge names
// what fired (the exit signal), never a bare "—" — a reader scanning the
// table must not have the seven most alarming rows advertise themselves
// with the emptiest-looking cell on the page. EXIT_ONLY is the internal
// enum only; see independenceLabel for the column that actually reads
// "—" (no independence measurement was made below the buyer floor).
func verdictLabel(v score.Verdict) string {
	if v == score.VerdictExitOnly {
		return string(score.VerdictDistributing)
	}
	return string(v)
}

// independenceLabel is the N -> M column. Below the buyer floor
// (VerdictExitOnly) no independence measurement was made at all, so
// "1->1" would falsely claim a clustering result that never happened —
// docs/DESIGN.md's Output section says this column reads "—" instead.
// When coverage is partial (fewer than N buyers actually had funder data
// fetched) the column names it, e.g. "4->2 (3/4 checked)", so a CONFIRMED
// or UNVERIFIED reading is never mistaken for a fully-checked one.
func independenceLabel(tr pipeline.TokenReport) string {
	if tr.Verdict == score.VerdictExitOnly {
		return "—"
	}
	if tr.Covered < tr.N {
		return fmt.Sprintf("%d->%d (%d/%d checked)", tr.N, tr.M, tr.Covered, tr.N)
	}
	return fmt.Sprintf("%d->%d", tr.N, tr.M)
}

func signedUSD(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "-"
		v = -v
	}
	return fmt.Sprintf("%s$%.0f", sign, v)
}

// exchFlowColor is risk-based: a positive exchange_net_flow_usd means
// value moving into exchange addresses (deposits, sell-side pressure —
// red); negative means withdrawal (green).
func exchFlowColor(v float64) string {
	if v < 0 {
		return ansiGreen
	}
	return ansiRed
}

// smartFlowColor is direction-based, not risk-based: smart_trader_net_flow_usd
// positive means the cohort under evaluation is net buying (green),
// negative means net selling (red). Deliberately the opposite mapping
// from exchFlowColor — the two columns answer different questions, and
// using one palette for both would make e.g. SOL's positive (buying)
// smart flow read as alarming, and USDC's negative (selling) smart flow
// read as reassuring, which is backwards.
func smartFlowColor(v float64) string {
	if v < 0 {
		return ansiRed
	}
	return ansiGreen
}

func printPlan(items []pipeline.PlanItem) {
	fmt.Println(colorize("Call plan (no HTTP made yet):", ansiBold))
	fmt.Printf("%-32s %8s %8s %8s %8s %10s\n", "endpoint", "total", "cached", "new", "cost/ea", "new credits")
	totalNew, totalCredits := 0, 0
	for _, it := range items {
		fmt.Printf("%-32s %8d %8d %8d %8d %10d\n", it.Endpoint, it.Total, it.Cached, it.New, it.CostPerCall, it.NewCreditsEstimate())
		totalNew += it.New
		totalCredits += it.NewCreditsEstimate()
	}
	fmt.Println(strings.Repeat("-", 78))
	fmt.Printf("%-32s %8s %8s %8d %8s %10d\n", "TOTAL", "", "", totalNew, "", totalCredits)
}

// censusHeadline is docs/DESIGN.md's Output requirement that the census
// lead as "a sentence a viewer can read in one beat, naming the funders
// and their wallet counts" rather than sitting as a small stats box a
// viewer has to parse.
func censusHeadline(census score.FunderCensus, walletCount int) string {
	var traced int
	var parts []string
	for _, fc := range census.TopFunders {
		if fc.WalletCount <= 1 {
			break
		}
		traced += fc.WalletCount
		label := fc.Label
		if label == "" {
			label = fc.Address
		}
		parts = append(parts, fmt.Sprintf("%s funded %d", label, fc.WalletCount))
	}
	if len(parts) == 0 {
		return fmt.Sprintf("%d distinct First Funders across %d wallets — none shared by more than one wallet.",
			census.DistinctFunders, walletCount)
	}
	who := "person"
	if len(parts) != 1 {
		who = "people"
	}

	// Name only the biggest few. Listing all of them runs past a screen's width
	// and reads as a bug when several share a Nansen label -- the full seed
	// produces four separate addresses all labelled "High Activity", so the
	// sentence ended "... High Activity funded 2, High Activity funded 2, ...".
	// The count carries the finding; the tail belongs in the table below.
	const named = 3
	funders := len(parts)
	tail := ""
	if funders > named {
		tail = fmt.Sprintf(", and %d more", funders-named)
		parts = parts[:named]
	}
	return fmt.Sprintf("%d of %d top smart-money wallets trace back to just %d %s: %s%s.",
		traced, walletCount, funders, who, strings.Join(parts, ", "), tail)
}

func printTable(result *pipeline.Result) {
	fmt.Println(colorize(censusHeadline(result.Census, result.WalletCount), ansiBold))

	fmt.Println()
	shown := make([]pipeline.TokenReport, 0, len(result.TokenReports))
	weak := 0
	for _, tr := range result.TokenReports {
		if tr.Verdict == score.VerdictWeak {
			weak++
			continue
		}
		shown = append(shown, tr)
	}
	fmt.Println(colorize(fmt.Sprintf("Tokens (%d scored) — the product: is the smart-money signal real?", len(shown)), ansiBold+ansiCyan))
	fmt.Printf("%-4s %-24s %-12s %8s %14s %14s\n", "#", "symbol", "verdict", "N -> M", "smart_flow", "exch_flow")
	for i, tr := range shown {
		fmt.Printf("%-4d %-24s %s %8s %s %s\n",
			i+1, truncateStr(tr.Symbol, 24),
			colorize(fmt.Sprintf("%-12s", verdictLabel(tr.Verdict)), verdictColor(tr.Verdict)),
			independenceLabel(tr),
			colorize(fmt.Sprintf("%14s", signedUSD(tr.SmartTraderNetFlowUSD)), smartFlowColor(tr.SmartTraderNetFlowUSD)),
			colorize(fmt.Sprintf("%14s", signedUSD(tr.ExchangeNetFlowUSD)), exchFlowColor(tr.ExchangeNetFlowUSD)))
	}
	if weak > 0 {
		fmt.Println(colorize(fmt.Sprintf("%d tokens had fewer than 3 smart-money buyers — no independence signal", weak), ansiGray))
	}

	fmt.Println()
	fmt.Println(colorize(fmt.Sprintf("Wallets (%d scored) — supporting evidence", len(result.RankedWallets)), ansiBold+ansiCyan))
	fmt.Printf("%-4s %-44s %-24s %8s %10s %8s\n", "#", "address", "label", "score", "pnl_usd", "win_rate")
	topWallets := result.RankedWallets
	if len(topWallets) > 20 {
		topWallets = topWallets[:20]
	}
	walletByAddr := map[string]struct{ pnl, wr float64 }{}
	for _, w := range result.Wallets {
		walletByAddr[w.Address] = struct{ pnl, wr float64 }{w.RealizedPnLUSD, w.WinRate}
	}
	for i, w := range topWallets {
		extra := walletByAddr[w.Address]
		fmt.Printf("%-4d %-44s %-24s %s %10.0f %8.2f\n",
			i+1, w.Address, truncateStr(w.Label, 24),
			colorize(fmt.Sprintf("%8.3f", w.Score), scoreColor(w.Score)),
			extra.pnl, extra.wr)
	}

	if len(result.Failures) > 0 {
		fmt.Println()
		fmt.Println(colorize(fmt.Sprintf("%d partial failures (see stderr)", len(result.Failures)), ansiGray))
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
