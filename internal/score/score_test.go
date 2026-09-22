package score

import (
	"math"
	"testing"
	"time"
)

var ref = time.Date(2026, 9, 22, 0, 0, 0, 0, time.UTC)

func f(v float64) *float64 { return &v }

func TestRankNormalize(t *testing.T) {
	cases := []struct {
		name string
		in   []float64
		want []float64
	}{
		{"empty", nil, nil},
		{"single", []float64{7}, []float64{0.5}},
		{"all equal", []float64{3, 3, 3}, []float64{0.5, 0.5, 0.5}},
		{"spread", []float64{0, 5, 10}, []float64{0, 0.5, 1}},
		{"negative spread", []float64{-10, 0, 10}, []float64{0, 0.5, 1}},
		{"tie shares average rank", []float64{1, 1, 3}, []float64{0.25, 0.25, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := rankNormalize(c.in)
			if len(got) != len(c.want) {
				t.Fatalf("length mismatch: got %v want %v", got, c.want)
			}
			for i := range got {
				if math.Abs(got[i]-c.want[i]) > 1e-9 {
					t.Fatalf("index %d: got %v want %v", i, got, c.want)
				}
			}
		})
	}
}

// TestRankNormalize_AntiSaturation is the regression the DESIGN.md bug
// report demands: a single extreme outlier must not collapse every other
// distinct value to the same normalised score, the way linear min-max
// scaling did (every rendered token scored exactly 1.000).
func TestRankNormalize_AntiSaturation(t *testing.T) {
	got := rankNormalize([]float64{1, 2, 3, 4, 1_000_000})
	seen := map[float64]bool{}
	for _, v := range got[:4] {
		if seen[v] {
			t.Fatalf("distinct inputs produced a repeated normalised output: %v", got)
		}
		seen[v] = true
	}
	if got[4] != 1 {
		t.Fatalf("expected the outlier to rank highest, got %v", got)
	}
}

func TestScoreWallets_RankingOrder(t *testing.T) {
	wallets := []WalletInput{
		{
			Address: "high", Label: "Smart Trader",
			RealizedPnLUSD: 100000, WinRate: 0.9,
			LastTradeTime: ref.Add(-1 * time.Hour),
			BoughtTokens:  []WalletTokenTrade{{TokenKey: "t1", TradeValueUSD: 1000}},
		},
		{
			Address: "low", Label: "Loser",
			RealizedPnLUSD: -50000, WinRate: 0.1,
			LastTradeTime: ref.Add(-1000 * time.Hour),
			BoughtTokens:  []WalletTokenTrade{{TokenKey: "t1", TradeValueUSD: 1000}},
		},
	}
	tokens := []TokenInput{
		{Key: "t1", SmartTraderNetFlowUSD: 1000, LiquidityUSD: f(10000)},
	}
	out := ScoreWallets(wallets, tokens, ref)
	if len(out) != 2 {
		t.Fatalf("expected 2 results, got %d", len(out))
	}
	if out[0].Address != "high" {
		t.Fatalf("expected 'high' to rank first, got %+v", out)
	}
	if out[0].Score <= out[1].Score {
		t.Fatalf("expected strictly descending score, got %v then %v", out[0].Score, out[1].Score)
	}
}

func TestScoreWallets_EmptyCohort(t *testing.T) {
	if out := ScoreWallets(nil, nil, ref); out != nil {
		t.Fatalf("expected nil for empty cohort, got %+v", out)
	}
}

func TestScoreWallets_SingleWallet(t *testing.T) {
	wallets := []WalletInput{
		{Address: "solo", WinRate: 0.5, RealizedPnLUSD: 100, LastTradeTime: ref},
	}
	out := ScoreWallets(wallets, nil, ref)
	if len(out) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out))
	}
	// Single-member cohort: every normalised term lands on the flat 0.5,
	// so the raw score is exactly the sum of weights.
	want := weightWinRate*0.5 + weightPnL*0.5 + weightRecency*0.5 + weightConcentration*0.5
	if math.Abs(out[0].Score-want) > 1e-9 {
		t.Fatalf("got score %v want %v", out[0].Score, want)
	}
}

func TestScoreWallets_ZeroAndNegativePnL(t *testing.T) {
	wallets := []WalletInput{
		{Address: "zero", RealizedPnLUSD: 0, WinRate: 0.5, LastTradeTime: ref},
		{Address: "neg", RealizedPnLUSD: -1000, WinRate: 0.5, LastTradeTime: ref},
		{Address: "pos", RealizedPnLUSD: 1000, WinRate: 0.5, LastTradeTime: ref},
	}
	out := ScoreWallets(wallets, nil, ref)
	byAddr := map[string]WalletScore{}
	for _, w := range out {
		byAddr[w.Address] = w
	}
	if !(byAddr["neg"].PnLNorm < byAddr["zero"].PnLNorm && byAddr["zero"].PnLNorm < byAddr["pos"].PnLNorm) {
		t.Fatalf("expected neg < zero < pos PnL norm, got neg=%v zero=%v pos=%v",
			byAddr["neg"].PnLNorm, byAddr["zero"].PnLNorm, byAddr["pos"].PnLNorm)
	}
}

func TestScoreWallets_WinRateBounds(t *testing.T) {
	wallets := []WalletInput{
		{Address: "w0", WinRate: 0, RealizedPnLUSD: 1, LastTradeTime: ref},
		{Address: "w1", WinRate: 1, RealizedPnLUSD: 1, LastTradeTime: ref},
	}
	out := ScoreWallets(wallets, nil, ref)
	byAddr := map[string]WalletScore{}
	for _, w := range out {
		byAddr[w.Address] = w
	}
	if byAddr["w0"].WinRateNorm != 0 {
		t.Fatalf("expected win_rate=0 to normalise to 0, got %v", byAddr["w0"].WinRateNorm)
	}
	if byAddr["w1"].WinRateNorm != 1 {
		t.Fatalf("expected win_rate=1 to normalise to 1, got %v", byAddr["w1"].WinRateNorm)
	}
}

// TestScoreWallets_ExchangeDistributionPenalty pins the sign convention
// from docs/DESIGN.md: exchange_net_flow_usd is net flow FOR exchange
// addresses, so a POSITIVE value means value moving INTO exchanges
// (deposits, sell-side pressure / distribution), not negative.
func TestScoreWallets_ExchangeDistributionPenalty(t *testing.T) {
	tokens := []TokenInput{
		{Key: "clean", SmartTraderNetFlowUSD: 100, ExchangeNetFlowUSD: -500, FlowAvailable: true, LiquidityUSD: f(1000)},
		{Key: "dumped", SmartTraderNetFlowUSD: 100, ExchangeNetFlowUSD: 500, FlowAvailable: true, LiquidityUSD: f(1000)},
	}
	wallets := []WalletInput{
		{
			Address: "clean-buyer", WinRate: 0.5, RealizedPnLUSD: 100, LastTradeTime: ref,
			BoughtTokens: []WalletTokenTrade{{TokenKey: "clean", TradeValueUSD: 1000}},
		},
		{
			Address: "dumped-buyer", WinRate: 0.5, RealizedPnLUSD: 100, LastTradeTime: ref,
			BoughtTokens: []WalletTokenTrade{{TokenKey: "dumped", TradeValueUSD: 1000}},
		},
	}
	out := ScoreWallets(wallets, tokens, ref)
	byAddr := map[string]WalletScore{}
	for _, w := range out {
		byAddr[w.Address] = w
	}
	if byAddr["dumped-buyer"].DistributionPenalty <= byAddr["clean-buyer"].DistributionPenalty {
		t.Fatalf("expected buyer of a distributing token to carry a larger penalty: clean=%v dumped=%v",
			byAddr["clean-buyer"].DistributionPenalty, byAddr["dumped-buyer"].DistributionPenalty)
	}
	if byAddr["dumped-buyer"].DistributionPenalty != distributionPenaltyMax {
		t.Fatalf("expected fully-flagged buyer to hit max penalty, got %v", byAddr["dumped-buyer"].DistributionPenalty)
	}
	if byAddr["clean-buyer"].DistributionPenalty != 0 {
		t.Fatalf("expected clean buyer to have zero penalty, got %v", byAddr["clean-buyer"].DistributionPenalty)
	}
}

func TestScoreWallets_ClusterPenalty(t *testing.T) {
	// 10 wallets all share funder "F", giving a cluster at
	// clusterPenaltyFullAt (saturation); "solo" has no funder data at all.
	wallets := []WalletInput{
		{Address: "solo", WinRate: 0.5, RealizedPnLUSD: 100, LastTradeTime: ref},
	}
	for i := 0; i < 10; i++ {
		addr := string(rune('a' + i))
		wallets = append(wallets, WalletInput{
			Address: "cluster-" + addr, WinRate: 0.5, RealizedPnLUSD: 100, LastTradeTime: ref,
			Funders: []Funder{{Address: "F"}},
		})
	}
	out := ScoreWallets(wallets, nil, ref)
	byAddr := map[string]WalletScore{}
	for _, w := range out {
		byAddr[w.Address] = w
	}
	if byAddr["cluster-a"].ClusterPenalty != clusterPenaltyMax {
		t.Fatalf("expected saturated cluster penalty for a 10-member cluster, got %v (size %d)",
			byAddr["cluster-a"].ClusterPenalty, byAddr["cluster-a"].ClusterSize)
	}
	if byAddr["solo"].ClusterPenalty != 0 {
		t.Fatalf("expected no cluster penalty for an unrelated wallet, got %v", byAddr["solo"].ClusterPenalty)
	}
	if byAddr["cluster-a"].Score >= byAddr["solo"].Score {
		t.Fatalf("expected clustered wallet to rank below solo wallet given identical raw stats")
	}
}
