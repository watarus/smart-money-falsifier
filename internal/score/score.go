// Package score implements the pure scoring functions from docs/DESIGN.md:
// no I/O, only computation over already-fetched wallet and token data, so
// it can be table-tested without any network or cache dependency.
//
// The token table is now categorical (see verdict.go, cluster.go): the
// previous continuous token score saturated (every rendered token scored
// exactly 1.000), which is how that normalisation bug hid in plain sight.
// The wallet ranking that remains is rank-based within the cohort, never
// magnitude-based, for the same reason.
package score

import (
	"math"
	"sort"
	"time"
)

// WalletInput is everything the scorer needs about one wallet, already
// fetched from pnl-summary, related-wallets, and its own trades from the
// seed.
type WalletInput struct {
	Address string
	Label   string

	RealizedPnLUSD float64
	WinRate        float64 // 0..1

	// Funders holds the wallet's First-Funder edges only (already filtered
	// by internal/pipeline to relation == "First Funder"; every other
	// relation, e.g. Deployed Program, is a contract artefact and must
	// never reach here). Used for both the cluster-collapse penalty below
	// and internal/score's Cluster/FunderCensus functions.
	Funders []Funder

	// FunderChecked is true when related-wallets actually succeeded for
	// this wallet (see BuyerInput.FunderChecked); it feeds Cluster's
	// coverage count so absent data can't manufacture a CONFIRMED verdict.
	FunderChecked bool

	// LastTradeTime is the wallet's most recent trade timestamp, used for
	// the recency term.
	LastTradeTime time.Time

	// BoughtTokens lists the tokens this wallet bought, used for the
	// exchange-distribution penalty and concentration term. TokenKey must
	// match a key in the token map passed to ScoreWallets.
	BoughtTokens []WalletTokenTrade
}

// WalletTokenTrade is one (wallet, token) bought-side aggregate.
type WalletTokenTrade struct {
	TokenKey      string
	TradeValueUSD float64
}

// TokenInput is everything the scorer needs about one token, already
// fetched from tgm/flow-intelligence (and, where available,
// tgm/token-information).
type TokenInput struct {
	Key string // e.g. "chain:address"

	Symbol string
	Name   string

	SmartTraderNetFlowUSD float64
	ExchangeNetFlowUSD    float64 // positive = value moving INTO exchange addresses (sell-side pressure)

	// FlowAvailable is false when tgm/flow-intelligence returned no row
	// for this token; a missing flow signal must never manufacture a
	// DISTRIBUTING verdict (see verdict.go).
	FlowAvailable bool

	// LiquidityUSD comes from tgm/token-information; nil if unavailable.
	LiquidityUSD *float64
}

// WalletScore is one ranked wallet result.
type WalletScore struct {
	Address string
	Label   string
	Score   float64

	WinRateNorm       float64
	PnLNorm           float64
	RecencyNorm       float64
	ConcentrationNorm float64

	DistributionPenalty float64
	ClusterPenalty      float64
	ClusterSize         int // count of wallets (in the scored cohort) this one collapses with, including itself
}

// weights from docs/DESIGN.md, renormalised after dropping the token-
// quality term (verdicts replace token scoring; there is no longer a
// per-wallet "mean token score" to blend in):
// score = 0.45*win_rate + 0.30*log_scaled(pnl) + 0.125*recency + 0.125*concentration
const (
	weightWinRate       = 0.45
	weightPnL           = 0.30
	weightRecency       = 0.125
	weightConcentration = 0.125

	// Penalty magnitudes: subtracted from the raw weighted score before
	// clamping to [0, 1]. Chosen so a wallet exhibiting the worst of
	// either signal loses meaningfully but a clean wallet is unaffected.
	distributionPenaltyMax = 0.15
	clusterPenaltyMax      = 0.10

	// clusterPenaltyThreshold: cluster sizes at or below this (a wallet
	// alone, or paired with just one other) are treated as normal;
	// unrelated wallets accumulate no penalty.
	clusterPenaltyThreshold = 2
	// clusterPenaltyFullAt: cluster size at or above which the penalty
	// saturates at its max.
	clusterPenaltyFullAt = 10
)

// ScoreWallets computes the conviction score for each wallet, given the
// token inputs (for the exchange-distribution penalty) and a reference
// time for the recency term (pass the seed's max block_timestamp, not
// time.Now, so scoring stays pure and deterministic).
func ScoreWallets(wallets []WalletInput, tokens []TokenInput, ref time.Time) []WalletScore {
	if len(wallets) == 0 {
		return nil
	}

	distributing := map[string]bool{}
	for _, t := range tokens {
		distributing[t.Key] = Distributing(t.SmartTraderNetFlowUSD, t.ExchangeNetFlowUSD, t.FlowAvailable)
	}

	// Cluster the whole scored cohort (not per-token) to find each
	// wallet's collapse group for the cluster penalty; this is separate
	// from the per-token cluster collapse the report renders.
	buyers := make([]BuyerInput, len(wallets))
	for i, w := range wallets {
		buyers[i] = BuyerInput{Address: w.Address, Label: w.Label, Funders: w.Funders}
	}
	clusterSize := make(map[string]int, len(wallets))
	for _, c := range Cluster(buyers).Clusters {
		for _, m := range c.Members {
			clusterSize[m.Address] = len(c.Members)
		}
	}

	winRates := make([]float64, len(wallets))
	pnlLog := make([]float64, len(wallets))
	recency := make([]float64, len(wallets))
	concentration := make([]float64, len(wallets))

	for i, w := range wallets {
		winRates[i] = clamp01(w.WinRate)
		pnlLog[i] = signedLog1p(w.RealizedPnLUSD)
		recency[i] = recencyRaw(w.LastTradeTime, ref)
		concentration[i] = concentrationRaw(w.BoughtTokens)
	}

	winRateNorm := rankNormalize(winRates)
	pnlNorm := rankNormalize(pnlLog)
	recencyNorm := rankNormalize(recency)
	concentrationNorm := rankNormalize(concentration)

	out := make([]WalletScore, len(wallets))
	for i, w := range wallets {
		distPenalty := distributionPenalty(w.BoughtTokens, distributing) * distributionPenaltyMax
		size := clusterSize[w.Address]
		clusterPenalty := clusterPenaltyFor(size) * clusterPenaltyMax

		raw := weightWinRate*winRateNorm[i] +
			weightPnL*pnlNorm[i] +
			weightRecency*recencyNorm[i] +
			weightConcentration*concentrationNorm[i] -
			distPenalty - clusterPenalty

		out[i] = WalletScore{
			Address:             w.Address,
			Label:               w.Label,
			Score:               clamp01(raw),
			WinRateNorm:         winRateNorm[i],
			PnLNorm:             pnlNorm[i],
			RecencyNorm:         recencyNorm[i],
			ConcentrationNorm:   concentrationNorm[i],
			DistributionPenalty: distPenalty,
			ClusterPenalty:      clusterPenalty,
			ClusterSize:         size,
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Address < out[j].Address // deterministic tiebreak
	})
	return out
}

// distributionPenalty is the fraction of a wallet's bought trade value
// sitting in tokens whose distribution signal fires (smart traders net
// buying while exchange-labelled addresses net receive — see verdict.go).
func distributionPenalty(bought []WalletTokenTrade, distributing map[string]bool) float64 {
	var total, flagged float64
	for _, b := range bought {
		total += b.TradeValueUSD
		if distributing[b.TokenKey] {
			flagged += b.TradeValueUSD
		}
	}
	if total <= 0 {
		return 0
	}
	return flagged / total
}

// clusterPenaltyFor scales with the wallet's cluster-collapse size
// (docs/DESIGN.md's cluster.go union, run over the whole scored cohort),
// saturating at clusterPenaltyFullAt. A singleton or pair scores 0; this
// mirrors DESIGN's "the buyers may not be independent" caveat directly
// from the same funder-graph signal the token table renders, rather than
// from a raw related-wallets count that would include Deployed-* noise.
func clusterPenaltyFor(size int) float64 {
	if size <= clusterPenaltyThreshold {
		return 0
	}
	span := float64(clusterPenaltyFullAt - clusterPenaltyThreshold)
	frac := float64(size-clusterPenaltyThreshold) / span
	if frac > 1 {
		frac = 1
	}
	return frac
}

// recencyRaw converts a last-trade timestamp into a raw recency signal:
// larger is more recent. Zero-value (unknown) timestamps get the lowest
// possible raw value so normalisation ranks them last, not first.
func recencyRaw(last, ref time.Time) float64 {
	if last.IsZero() {
		return math.Inf(-1)
	}
	age := ref.Sub(last).Hours()
	if age < 0 {
		age = 0
	}
	return -age
}

// concentrationRaw scores fewer, larger positions over spray-and-pray:
// a Herfindahl-style share concentration (0..1, higher = more
// concentrated) over the wallet's bought trade value.
func concentrationRaw(bought []WalletTokenTrade) float64 {
	if len(bought) == 0 {
		return 0
	}
	var total float64
	byToken := map[string]float64{}
	for _, b := range bought {
		byToken[b.TokenKey] += b.TradeValueUSD
		total += b.TradeValueUSD
	}
	if total <= 0 {
		return 0
	}
	var hhi float64
	for _, v := range byToken {
		share := v / total
		hhi += share * share
	}
	return hhi
}

func signedLog1p(v float64) float64 {
	if v >= 0 {
		return math.Log1p(v)
	}
	return -math.Log1p(-v)
}

func clamp01(v float64) float64 {
	if math.IsNaN(v) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// rankNormalize scales values to [0, 1] by rank within the cohort, not by
// magnitude: the previous minMaxNormalize scaled linearly between the
// cohort's min and max, which is exactly how one outlier saturates every
// other member of the cohort to the same normalised value (the bug that
// made all 20 rendered tokens score 1.000). Ties share the average rank.
// -Inf sentinels (unknown recency) always rank last. Degenerate cases
// (empty input, a single value, or every value identical) all map to a
// flat 0.5: there's no basis to prefer one member of an undifferentiated
// cohort over another.
func rankNormalize(vals []float64) []float64 {
	out := make([]float64, len(vals))
	n := len(vals)
	if n == 0 {
		return out
	}
	if n == 1 {
		out[0] = 0.5
		return out
	}

	type indexed struct {
		i int
		v float64
	}
	sorted := make([]indexed, n)
	for i, v := range vals {
		sorted[i] = indexed{i: i, v: v}
	}
	sort.Slice(sorted, func(a, b int) bool {
		// -Inf always sorts lowest, otherwise ascending by value.
		if math.IsInf(sorted[a].v, -1) != math.IsInf(sorted[b].v, -1) {
			return math.IsInf(sorted[a].v, -1)
		}
		return sorted[a].v < sorted[b].v
	})

	allEqual := true
	for i := 1; i < n; i++ {
		if sorted[i].v != sorted[0].v {
			allEqual = false
			break
		}
	}
	if allEqual {
		for i := range out {
			out[i] = 0.5
		}
		return out
	}

	// Assign average rank (0-indexed) for ties, then scale by (n-1).
	i := 0
	for i < n {
		j := i
		for j+1 < n && sorted[j+1].v == sorted[i].v {
			j++
		}
		avgRank := float64(i+j) / 2
		for k := i; k <= j; k++ {
			out[sorted[k].i] = avgRank / float64(n-1)
		}
		i = j + 1
	}
	return out
}
