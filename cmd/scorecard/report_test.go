package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/watarus/nansen/internal/nansen"
	"github.com/watarus/nansen/internal/pipeline"
	"github.com/watarus/nansen/internal/score"
)

// TestWriteReport_AllVerdicts exercises html/template execution (which
// only surfaces errors at Execute time, not Parse time) against a
// synthetic Result covering every verdict label, including a cluster with
// a shared funder, one collapsed via a funder-is-a-buyer edge, one with no
// funder data, and a token with no flow-intelligence data at all.
func TestWriteReport_AllVerdicts(t *testing.T) {
	liquidity := 12345.0
	result := &pipeline.Result{
		Seed:        &nansen.DexTradesResponse{Data: make([]nansen.DexTrade, 3)},
		WalletCount: 4,
		TokenCount:  5,
		RankedWallets: []score.WalletScore{
			{Address: "wallet-a", Label: "Smart Trader", Score: 0.9},
			{Address: "wallet-b", Label: "", Score: 0.3},
		},
		Wallets: []score.WalletInput{
			{Address: "wallet-a", Label: "Smart Trader", RealizedPnLUSD: 1000, WinRate: 0.6},
			{Address: "wallet-b", RealizedPnLUSD: -200, WinRate: 0.2},
		},
		Census: score.FunderCensus{
			DistinctFunders: 3,
			SharedFunders:   1,
			TopFunders: []score.FunderCount{
				{Address: "funder-1", Label: "realkingof.sol", WalletCount: 2},
			},
		},
		TokenReports: []pipeline.TokenReport{
			{
				Key: "solana:tok-both", Symbol: "BOTH", Name: "Both Token",
				Verdict: score.VerdictBoth, N: 3, M: 1,
				SmartTraderNetFlowUSD: 500, ExchangeNetFlowUSD: 500, FlowAvailable: true,
				LiquidityUSD: &liquidity,
				Clusters: []score.ClusterGroup{{
					Members:      []score.BuyerInput{{Address: "wallet-a", Label: "Smart Trader", PnLUSD: 1000, WinRate: 0.6}},
					SharedFunder: &score.Funder{Address: "funder-1", Label: "realkingof.sol"},
				}},
			},
			{
				Key: "solana:tok-thin", Symbol: "THIN", Name: "Thin Token",
				Verdict: score.VerdictThin, N: 3, M: 1,
				SmartTraderNetFlowUSD: -100, ExchangeNetFlowUSD: -50, FlowAvailable: true,
				Clusters: []score.ClusterGroup{{
					Members:  []score.BuyerInput{{Address: "wallet-b", PnLUSD: -200, WinRate: 0.2}},
					ViaBuyer: "wallet-a",
				}},
			},
			{
				Key: "ethereum:tok-dist", Symbol: "DIST", Name: "Distributing Token",
				Verdict: score.VerdictDistributing, N: 4, M: 4,
				SmartTraderNetFlowUSD: 200, ExchangeNetFlowUSD: 300, FlowAvailable: true,
				Clusters: []score.ClusterGroup{{
					Members:      []score.BuyerInput{{Address: "wallet-a", PnLUSD: 1000, WinRate: 0.6}},
					NoFunderData: true,
				}},
			},
			{
				// FlowAvailable: false here (rather than on the WEAK token
				// below) so the "no flow-intelligence data" note is still
				// exercised on a row that actually renders — WEAK tokens
				// are collapsed behind the disclosure line and never reach
				// the template's per-token block.
				Key: "ethereum:tok-confirmed", Symbol: "CONF", Name: "Confirmed Token",
				Verdict: score.VerdictConfirmed, N: 5, M: 5,
				FlowAvailable: false,
				Clusters: []score.ClusterGroup{{
					Members: []score.BuyerInput{{Address: "wallet-a", PnLUSD: 1000, WinRate: 0.6}},
				}},
			},
			{
				Key: "ethereum:tok-weak", Symbol: "WEAKTOK", Name: "",
				Verdict: score.VerdictWeak, N: 1, M: 1,
				FlowAvailable: false,
				Clusters: []score.ClusterGroup{{
					Members:      []score.BuyerInput{{Address: "wallet-b", PnLUSD: -200, WinRate: 0.2}},
					NoFunderData: true,
				}},
			},
			{
				// N < 3 (no independence signal) but the exit signal fires
				// anyway: docs/DESIGN.md's buyer floor gates the
				// independence verdict only, never the row, so this must
				// still occupy a row with a "—" independence badge rather
				// than being swept behind the disclosure line with the
				// WEAK token above.
				Key: "solana:tok-exit", Symbol: "EXITONLY", Name: "Exit Only Token",
				Verdict: score.VerdictExitOnly, N: 1, M: 1,
				SmartTraderNetFlowUSD: 900, ExchangeNetFlowUSD: 400, FlowAvailable: true,
				Clusters: []score.ClusterGroup{{
					Members:      []score.BuyerInput{{Address: "wallet-a", PnLUSD: 1000, WinRate: 0.6}},
					NoFunderData: true,
				}},
			},
		},
	}

	path := filepath.Join(t.TempDir(), "report.html")
	if err := writeReport(path, result); err != nil {
		t.Fatalf("writeReport: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading report: %v", err)
	}
	html := string(raw)

	for _, want := range []string{
		"BOTH", "THIN", "DISTRIBUTING", "CONFIRMED",
		"shared funder realkingof.sol",
		"funder is itself a buyer: wallet-a",
		"no funder data (not evidence of independence)",
		"no flow-intelligence data for this token",
		"realkingof.sol",
		// docs/DESIGN.md Output: N < 3 tokens are collapsed behind a
		// disclosure line, not rendered as a WEAK row.
		"1 tokens had fewer than 3 smart-money buyers",
		// the census headline sentence, not just a raw stats box.
		"2 of 4 top smart-money wallets trace back to just 1 person",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report missing expected content %q", want)
		}
	}
	if strings.Contains(html, "WEAKTOK") {
		t.Errorf("report rendered the WEAK token %q in the main table; it must be collapsed behind the disclosure line", "WEAKTOK")
	}
	if !strings.Contains(html, "EXITONLY") {
		t.Errorf("report dropped the EXIT_ONLY token; the buyer floor must gate the independence verdict only, never the row")
	}
	if !strings.Contains(html, `<span class="verdict v-bad">—</span>`) {
		t.Errorf("report did not render the EXIT_ONLY token's independence badge as \"—\"")
	}
}
