package main

import (
	"fmt"
	"html/template"
	"os"

	"github.com/watarus/nansen/internal/pipeline"
	"github.com/watarus/nansen/internal/score"
)

type reportWalletRow struct {
	Rank     int
	Address  string
	Label    string
	Score    float64
	ScorePct int
	PnLUSD   float64
	WinRate  float64
}

type reportMemberRow struct {
	Address string
	Label   string
	PnLUSD  float64
	WinRate float64
}

type reportClusterRow struct {
	Size       int
	FunderDesc string
	Members    []reportMemberRow
}

type reportTokenRow struct {
	Rank              int
	Symbol            string
	Name              string
	Verdict           string
	VerdictClass      string
	Independence      string
	IndependenceClass string
	SmartFlow         string
	SmartFlowClass    string
	ExchFlow          string
	ExchFlowClass     string
	FlowNote          string
	Liquidity         string
	Move              string
	MoveClass         string
	Clusters          []reportClusterRow
}

type reportFunderRow struct {
	Label       string
	Address     string
	WalletCount int
}

type reportData struct {
	GeneratedAt     string
	WalletCount     int
	TokenCount      int
	FailureCount    int
	SeedTradeCount  int
	DistinctFunders int
	SharedFunders   int
	CensusHeadline  string
	WeakTokenCount  int
	MinSignal       string
	TopFunders      []reportFunderRow
	Wallets         []reportWalletRow
	Tokens          []reportTokenRow
}

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Is That Smart-Money Signal Real?</title>
<style>
  :root {
    --bg: #0b0e14;
    --panel: #131826;
    --panel-border: #232a3d;
    --text: #e6e9f0;
    --muted: #8891a7;
    --accent: #6ee7b7;
    --accent-warn: #fbbf24;
    --accent-bad: #fb7185;
    --row-alt: #10141f;
  }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    background: var(--bg);
    color: var(--text);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    padding: 40px 48px 80px;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    border-bottom: 1px solid var(--panel-border);
    padding-bottom: 20px;
    margin-bottom: 24px;
    flex-wrap: wrap;
    gap: 16px;
  }
  h1 { font-size: 22px; margin: 0; letter-spacing: -0.01em; }
  .subtitle { color: var(--muted); font-size: 13px; margin-top: 4px; }
  .stats { display: flex; gap: 28px; }
  .stat { text-align: right; }
  .stat .n { font-size: 20px; font-weight: 600; font-variant-numeric: tabular-nums; }
  .stat .l { font-size: 11px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.04em; }
  .census {
    background: var(--panel);
    border: 1px solid var(--panel-border);
    border-radius: 10px;
    padding: 14px 18px;
    margin-bottom: 32px;
    font-size: 13px;
    color: var(--text);
  }
  .census .headline { color: var(--muted); margin-bottom: 6px; }
  .census .funder-row { display: flex; gap: 10px; padding: 2px 0; font-variant-numeric: tabular-nums; }
  .census .funder-row .label { color: var(--accent); min-width: 160px; }
  .census .funder-row .addr { color: var(--muted); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 12px; }
  section { margin-bottom: 40px; }
  h2 {
    font-size: 14px;
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--muted);
    margin: 0 0 14px;
  }
  table {
    width: 100%;
    border-collapse: collapse;
    background: var(--panel);
    border: 1px solid var(--panel-border);
    border-radius: 10px;
    overflow: hidden;
  }
  th, td {
    padding: 10px 14px;
    text-align: left;
    font-size: 13px;
    font-variant-numeric: tabular-nums;
  }
  th {
    color: var(--muted);
    font-weight: 500;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--panel-border);
  }
  tbody tr:nth-child(even) { background: var(--row-alt); }
  td.addr {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: var(--muted);
    font-size: 12px;
  }
  td.label {
    color: var(--accent);
    font-weight: 500;
  }
  td.num, th.num { text-align: right; }
  .bar-cell { display: flex; align-items: center; gap: 8px; justify-content: flex-end; }
  .bar-track { width: 80px; height: 6px; border-radius: 3px; background: #1c2233; overflow: hidden; }
  .bar-fill { height: 100%; border-radius: 3px; }
  .score-good { background: var(--accent); }
  .score-mid { background: var(--accent-warn); }
  .score-bad { background: var(--accent-bad); }
  .score-num { width: 42px; text-align: right; font-weight: 600; }
  .verdict {
    display: inline-block;
    padding: 3px 10px;
    border-radius: 999px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.03em;
  }
  .v-bad { background: rgba(251,113,133,0.16); color: var(--accent-bad); }
  .v-warn { background: rgba(251,191,36,0.16); color: var(--accent-warn); }
  .v-good { background: rgba(110,231,183,0.16); color: var(--accent); }
  .move { margin-top: 5px; font-size: 11px; font-weight: 600; color: var(--muted); line-height: 1.3; }
  .move.late { color: var(--accent-warn); }
  .v-flat { background: rgba(136,145,167,0.16); color: var(--muted); }
  /* exchange-flow column: risk-based — positive (deposits/sell pressure) red,
     negative (withdrawals) green. */
  .flow-pos { color: var(--accent-bad); }
  .flow-neg { color: var(--accent); }
  /* smart-trader-flow column: direction-based, not risk-based — positive
     (net buying) green, negative (net selling) red. Deliberately the
     opposite mapping from .flow-pos/.flow-neg: the two columns answer
     different questions, so one palette can't mean both. */
  .flow-buy { color: var(--accent); }
  .flow-sell { color: var(--accent-bad); }
  details.token-row summary {
    cursor: pointer;
    list-style: none;
  }
  details.token-row summary::-webkit-details-marker { display: none; }
  details.token-row { border-bottom: 1px solid var(--panel-border); }
  details.token-row:last-child { border-bottom: none; }
  .token-summary-grid {
    display: grid;
    grid-template-columns: 28px minmax(0,2fr) minmax(0,1fr) 90px minmax(0,1fr) minmax(0,1fr) minmax(0,1fr);
    align-items: center;
    gap: 12px;
    padding: 10px 14px;
    font-size: 13px;
  }
  .token-summary-grid:nth-child(odd) { background: var(--row-alt); }
  .symbol { font-weight: 600; }
  .name { color: var(--muted); font-size: 11px; }
  .cluster-detail { padding: 4px 14px 16px 42px; background: #0d111b; }
  .cluster-group {
    border: 1px solid var(--panel-border);
    border-radius: 8px;
    margin: 8px 0;
    overflow: hidden;
  }
  .cluster-head {
    background: #171d2e;
    padding: 6px 12px;
    font-size: 12px;
    color: var(--muted);
  }
  .cluster-head b { color: var(--text); }
  .no-flow { color: var(--muted); font-style: italic; }
  footer { color: var(--muted); font-size: 11px; margin-top: 40px; }
</style>
</head>
<body>
<header>
  <div>
    <h1>Is That Smart-Money Signal Real?</h1>
    <div class="subtitle">generated {{.GeneratedAt}} &middot; docs/DESIGN.md verdicts, not a copy-trade ranking</div>
  </div>
  <div class="stats">
    <div class="stat"><div class="n">{{.SeedTradeCount}}</div><div class="l">seed trades</div></div>
    <div class="stat"><div class="n">{{.WalletCount}}</div><div class="l">wallets</div></div>
    <div class="stat"><div class="n">{{.TokenCount}}</div><div class="l">tokens</div></div>
    <div class="stat"><div class="n">{{.FailureCount}}</div><div class="l">partial failures</div></div>
  </div>
</header>

<div class="census">
  <div class="headline">{{.CensusHeadline}}</div>
  {{range .TopFunders}}
  <div class="funder-row"><span class="label">{{if .Label}}{{.Label}}{{else}}(no label){{end}}</span><span class="addr">{{.Address}}</span><span>funds {{.WalletCount}} wallets</span></div>
  {{end}}
</div>

<section>
  <h2>Tokens &mdash; the product: is the signal real?</h2>
  <div style="background:var(--panel); border:1px solid var(--panel-border); border-radius:10px; overflow:hidden;">
    {{range .Tokens}}
    <details class="token-row">
      <summary>
        <div class="token-summary-grid">
          <div>{{.Rank}}</div>
          <div><div class="symbol">{{.Symbol}}</div>{{if .Name}}<div class="name">{{.Name}}</div>{{end}}</div>
          <div><span class="verdict {{.VerdictClass}}">{{.Verdict}}</span>{{if .Move}}<div class="move {{.MoveClass}}">{{.Move}}</div>{{end}}</div>
          <div><span class="verdict {{.IndependenceClass}}">{{.Independence}}</span></div>
          <div class="{{.SmartFlowClass}}">{{.SmartFlow}}</div>
          <div class="{{.ExchFlowClass}}">{{if .ExchFlow}}{{.ExchFlow}}{{else}}&mdash;{{end}}</div>
          <div>{{if .Liquidity}}{{.Liquidity}}{{else}}&mdash;{{end}}</div>
        </div>
      </summary>
      <div class="cluster-detail">
        {{if .FlowNote}}<div class="no-flow">{{.FlowNote}}</div>{{end}}
        {{range .Clusters}}
        <div class="cluster-group">
          <div class="cluster-head"><b>{{.Size}}</b> wallet{{if ne .Size 1}}s{{end}} &mdash; {{.FunderDesc}}</div>
          <table>
            <thead><tr><th>Address</th><th>Label</th><th class="num">Realized PnL</th><th class="num">Win Rate</th></tr></thead>
            <tbody>
              {{range .Members}}
              <tr>
                <td class="addr">{{.Address}}</td>
                <td class="label">{{if .Label}}{{.Label}}{{else}}&mdash;{{end}}</td>
                <td class="num">{{printf "%.0f" .PnLUSD}}</td>
                <td class="num">{{printf "%.1f" .WinRate}}%</td>
              </tr>
              {{end}}
            </tbody>
          </table>
        </div>
        {{end}}
      </div>
    </details>
    {{end}}
  </div>
  {{if .WeakTokenCount}}
  <div style="color:var(--muted); font-size:12px; margin-top:10px;">{{.WeakTokenCount}} tokens had too little smart-money buying to judge &mdash; fewer than 3 buyers or under {{.MinSignal}} bought &mdash; and no exit signal.</div>
  {{end}}
</section>

<section>
  <h2>Wallets &mdash; supporting evidence, not the headline</h2>
  <table>
    <thead>
      <tr>
        <th>#</th><th>Address</th><th>Label</th>
        <th class="num">Realized PnL (USD)</th><th class="num">Win Rate</th><th class="num">Score</th>
      </tr>
    </thead>
    <tbody>
      {{range .Wallets}}
      <tr>
        <td>{{.Rank}}</td>
        <td class="addr">{{.Address}}</td>
        <td class="label">{{if .Label}}{{.Label}}{{else}}&mdash;{{end}}</td>
        <td class="num">{{printf "%.0f" .PnLUSD}}</td>
        <td class="num">{{printf "%.1f" .WinRate}}%</td>
        <td class="num">
          <div class="bar-cell">
            <div class="bar-track"><div class="bar-fill {{scoreClass .Score}}" style="width:{{.ScorePct}}%"></div></div>
            <div class="score-num">{{printf "%.3f" .Score}}</div>
          </div>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
</section>

<footer>Falsifier scorecard &middot; verdicts per docs/DESIGN.md &middot; call evidence in out/calls.jsonl</footer>
</body>
</html>
`

func scoreClass(s float64) string {
	switch {
	case s >= 0.7:
		return "score-good"
	case s >= 0.4:
		return "score-mid"
	default:
		return "score-bad"
	}
}

// verdictClass maps a score.Verdict to a CSS badge class matching
// table.go's terminal colour convention: BOTH/DISTRIBUTING/EXIT_ONLY are
// the falsifier's exit signal firing (bad/red), THIN is a caution
// (warn/yellow), CONFIRMED survived both checks (good/green), WEAK is
// uninformative (flat/gray).
func verdictClass(v score.Verdict) string {
	switch v {
	case score.VerdictBoth, score.VerdictDistributing, score.VerdictExitOnly:
		return "v-bad"
	case score.VerdictThin:
		return "v-warn"
	case score.VerdictConfirmed:
		return "v-good"
	default: // CONCENTRATED, INDEPENDENT, UNVERIFIED, WEAK
		return "v-flat"
	}
}

// independenceClass badges the "—" independence column red on EXIT_ONLY:
// no independence measurement was made below the buyer floor, but the exit
// signal that put this row in the table at all is still exactly as
// alarming as a measured BOTH, so the badge must not read as neutral.
func independenceClass(v score.Verdict) string {
	if v == score.VerdictExitOnly {
		return "v-bad"
	}
	return "v-flat"
}

// verdictLabelHTML renders EXIT_ONLY as DISTRIBUTING — see table.go's
// verdictLabel for why: the verdict badge must name what fired, never a
// bare dash. EXIT_ONLY is the internal enum only.
func verdictLabelHTML(v score.Verdict) string {
	if v == score.VerdictExitOnly {
		return string(score.VerdictDistributing)
	}
	return string(v)
}

// independenceLabelHTML is the N -> M column — see table.go's
// independenceLabel: below the buyer floor (VerdictExitOnly) no
// independence measurement was made, so it reads "—" rather than a
// fabricated "1 -> 1".
func independenceLabelHTML(tr pipeline.TokenReport) string {
	if tr.Verdict == score.VerdictExitOnly {
		return "—"
	}
	if tr.Covered < tr.N {
		return fmt.Sprintf("%d → %d (%d/%d checked)", tr.N, tr.M, tr.Covered, tr.N)
	}
	return fmt.Sprintf("%d → %d", tr.N, tr.M)
}

func formatSignedUSD(v float64) string {
	sign := "+"
	if v < 0 {
		sign = "−" // minus sign, distinct from a hyphenated dash
		v = -v
	}
	return fmt.Sprintf("%s$%.0f", sign, v)
}

// exchFlowClass is risk-based — see table.go's exchFlowColor: positive
// exchange_net_flow_usd (deposits, sell-side pressure) is red, negative
// (withdrawals) is green.
func exchFlowClass(v float64) string {
	if v < 0 {
		return "flow-neg"
	}
	return "flow-pos"
}

// smartFlowClass is direction-based, not risk-based — see table.go's
// smartFlowColor: positive smart_trader_net_flow_usd (net buying) is
// green, negative (net selling) is red. Deliberately the opposite mapping
// from exchFlowClass; using one palette for both would make e.g. SOL's
// positive (buying) smart flow read as alarming and USDC's negative
// (selling) smart flow read as reassuring, which is backwards.
func smartFlowClass(v float64) string {
	if v < 0 {
		return "flow-sell"
	}
	return "flow-buy"
}

// formatUSD renders "" (the template's "—" fallback) for both a nil
// pointer and an exact zero. tgm/token-information's liquidity_usd is
// never actually absent in practice — verified live: it comes back as a
// literal 0.0, not null, for native-asset pseudo-addresses (ETH, SOL),
// which have no AMM pool of their own and so no liquidity metric applies.
// A real DEX-listed token with an active market essentially never reports
// exactly $0.00 liquidity, so treating 0 as "not meaningfully measured"
// here avoids printing a fabricated-looking zero for assets the metric
// doesn't apply to, without a false "missing data" claim for the (never
// observed) genuinely-absent case.
func formatUSD(v *float64) string {
	if v == nil || *v == 0 {
		return ""
	}
	return fmt.Sprintf("$%.0f", *v)
}

// funderDescription names why a cluster collapsed, per docs/DESIGN.md:
// a shared funder address, a funder that is itself another buyer, an
// independent singleton, or a singleton with no funder data at all —
// missing data must never be silently indistinguishable from
// independence.
func funderDescription(c score.ClusterGroup) string {
	switch {
	case c.SharedFunder != nil:
		label := c.SharedFunder.Label
		if label == "" {
			label = "unlabelled funder"
		}
		return fmt.Sprintf("shared funder %s (%s)", label, c.SharedFunder.Address)
	case c.ViaBuyer != "":
		return fmt.Sprintf("funder is itself a buyer: %s", c.ViaBuyer)
	case c.NoFunderData:
		return "no funder data (not evidence of independence)"
	default:
		return "independent (no shared funder)"
	}
}

func writeReport(path string, result *pipeline.Result) error {
	data := reportData{
		GeneratedAt:     nowRFC3339(),
		WalletCount:     result.WalletCount,
		TokenCount:      result.TokenCount,
		FailureCount:    len(result.Failures),
		SeedTradeCount:  len(result.Seed.Data),
		DistinctFunders: result.Census.DistinctFunders,
		SharedFunders:   result.Census.SharedFunders,
		CensusHeadline:  censusHeadline(result.Census, result.WalletCount),
		MinSignal:       fmt.Sprintf("$%.0f", result.MinSignalUSD),
	}

	for i, fc := range result.Census.TopFunders {
		if i >= 5 || fc.WalletCount <= 1 {
			break
		}
		data.TopFunders = append(data.TopFunders, reportFunderRow{Label: fc.Label, Address: fc.Address, WalletCount: fc.WalletCount})
	}

	walletByAddr := map[string]struct {
		pnl float64
		wr  float64
	}{}
	for _, w := range result.Wallets {
		walletByAddr[w.Address] = struct {
			pnl float64
			wr  float64
		}{w.RealizedPnLUSD, w.WinRate}
	}

	wallets := result.RankedWallets
	if len(wallets) > 20 {
		wallets = wallets[:20]
	}
	for i, w := range wallets {
		extra := walletByAddr[w.Address]
		data.Wallets = append(data.Wallets, reportWalletRow{
			Rank: i + 1, Address: w.Address, Label: w.Label,
			Score: w.Score, ScorePct: int(w.Score * 100),
			PnLUSD: extra.pnl, WinRate: extra.wr * 100,
		})
	}

	// The buyer floor gates the independence verdict only, never the row
	// (docs/DESIGN.md's Output section): only WEAK tokens — N < 3 buyers
	// AND no firing exit signal — are collapsed behind a disclosure line.
	// EXIT_ONLY tokens (N < 3 but the exit signal fires) still occupy a
	// row, just with a "—" independence badge.
	rank := 0
	for _, tr := range result.TokenReports {
		if tr.Verdict == score.VerdictWeak {
			data.WeakTokenCount++
			continue
		}
		rank++
		row := reportTokenRow{
			Rank: rank, Symbol: tr.Symbol, Name: tr.Name,
			Verdict: verdictLabelHTML(tr.Verdict), VerdictClass: verdictClass(tr.Verdict),
			Independence:      independenceLabelHTML(tr),
			IndependenceClass: independenceClass(tr.Verdict),
			SmartFlow:         formatSignedUSD(tr.SmartTraderNetFlowUSD),
			SmartFlowClass:    smartFlowClass(tr.SmartTraderNetFlowUSD),
			ExchFlow:          formatSignedUSD(tr.ExchangeNetFlowUSD),
			ExchFlowClass:     exchFlowClass(tr.ExchangeNetFlowUSD),
			Liquidity:         formatUSD(tr.LiquidityUSD),
			Move:              moveLabel(tr.Move),
			MoveClass:         moveClass(tr.Move),
		}
		switch {
		case !tr.FlowAvailable:
			row.FlowNote = "no flow-intelligence data for this token; the exit signal cannot fire"
			row.ExchFlow, row.ExchFlowClass = "", ""
		case !tr.ExchangeObserved:
			// No exchange address touched the token. A "+$0" here would read
			// as "checked, no inflow" when nothing was there to check.
			row.FlowNote = "no exchange address has touched this token; the exit check had nothing to test"
			row.ExchFlow, row.ExchFlowClass = "", ""
		}
		for _, c := range tr.Clusters {
			cr := reportClusterRow{Size: len(c.Members), FunderDesc: funderDescription(c)}
			for _, m := range c.Members {
				cr.Members = append(cr.Members, reportMemberRow{
					Address: m.Address, Label: m.Label, PnLUSD: m.PnLUSD, WinRate: m.WinRate * 100,
				})
			}
			row.Clusters = append(row.Clusters, cr)
		}
		data.Tokens = append(data.Tokens, row)
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{"scoreClass": scoreClass}).Parse(reportTemplate)
	if err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("executing report template: %w", err)
	}
	return nil
}
