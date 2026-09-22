// Package pipeline orchestrates the seed -> wallet enrichment -> token
// enrichment fan-out described in docs/DESIGN.md, using a bounded worker
// pool over internal/nansen's cached client, and feeds the results into
// internal/score.
package pipeline

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/watarus/nansen/internal/nansen"
	"github.com/watarus/nansen/internal/score"
)

// DefaultDateFrom/DefaultDateTo pin the pnl-summary lookback window to the
// fixed range docs/DESIGN.md was designed against. This must stay a fixed
// value (not derived from time.Now): the pnl-summary cache key hashes the
// request body, so a date range that drifts with wall-clock time would
// make every cached pnl-summary response miss on a later run and re-spend
// all 172 credits, breaking the "re-runs cost zero credits" requirement.
const (
	DefaultDateFrom = "2026-06-22"
	DefaultDateTo   = "2026-09-22"
)

// Failure records one partial failure during fan-out; the pipeline
// collects these and continues rather than aborting the run.
type Failure struct {
	Stage string // "wallet:pnl-summary", "wallet:related-wallets", "token:flow-intelligence", "token:token-information"
	Key   string
	Err   error
}

// Pipeline runs seed -> wallet enrichment -> token enrichment.
type Pipeline struct {
	Client      *nansen.Client
	Concurrency int
	DateFrom    string
	DateTo      string
	// MaxWallets caps the enrichment universe to the N highest-conviction
	// wallets in the seed (0 = no cap). Used to look at real data cheaply
	// while iterating, instead of fanning out over the whole seed.
	MaxWallets int
}

// limitUniverse keeps only the MaxWallets wallets that bought the most value in
// the seed, and narrows the token universe to what those wallets touched.
//
// Map iteration order in Go is randomised, so the cut has to be by an explicit
// ranking or two runs would enrich different wallets and thrash the cache.
// Ties break on address to stay deterministic.
func limitUniverse(wallets map[string]*walletAgg, tokens map[string]*tokenAgg, max int) (map[string]*walletAgg, map[string]*tokenAgg) {
	if max <= 0 || len(wallets) <= max {
		return wallets, tokens
	}

	type ranked struct {
		addr  string
		value float64
	}
	order := make([]ranked, 0, len(wallets))
	for addr, w := range wallets {
		var total float64
		for _, t := range w.boughtTokens {
			total += t.TradeValueUSD
		}
		order = append(order, ranked{addr: addr, value: total})
	}
	sort.Slice(order, func(i, j int) bool {
		if order[i].value != order[j].value {
			return order[i].value > order[j].value
		}
		return order[i].addr < order[j].addr
	})

	keptWallets := make(map[string]*walletAgg, max)
	keptTokens := map[string]*tokenAgg{}
	for _, r := range order[:max] {
		w := wallets[r.addr]
		keptWallets[r.addr] = w
		for _, t := range w.boughtTokens {
			if agg, ok := tokens[t.TokenKey]; ok {
				keptTokens[t.TokenKey] = agg
			}
		}
	}
	return keptWallets, keptTokens
}

func New(client *nansen.Client, concurrency int) *Pipeline {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Pipeline{
		Client:      client,
		Concurrency: concurrency,
		DateFrom:    DefaultDateFrom,
		DateTo:      DefaultDateTo,
	}
}

// walletAgg accumulates per-wallet facts derived from the seed trades,
// before enrichment.
type walletAgg struct {
	address      string
	label        string
	lastTrade    time.Time
	boughtTokens []score.WalletTokenTrade
	// chains is every chain this wallet traded on in the seed.
	// related-wallets rejects chain:"all" (verified live: 422
	// invalid_field_value), so it must be called once per (wallet, chain)
	// pair; pnl-summary does accept "all" and is called once per wallet.
	chains map[string]struct{}
}

func walletChainKey(address, chain string) string { return address + "|" + chain }

func splitWalletChainKey(key string) (address, chain string, ok bool) {
	parts := strings.SplitN(key, "|", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// tokenAgg accumulates per-token facts derived from the seed trades
// (chain+address identifies a token, since the same address can exist on
// multiple chains), before enrichment.
type tokenAgg struct {
	chain   string
	address string
	// seedSymbol is the token_bought_symbol/token_sold_symbol seen in the
	// seed, kept as a fallback token label if tgm/token-information didn't
	// resolve a symbol (or wasn't fetched, e.g. offline cache miss).
	seedSymbol string
}

func tokenKey(chain, address string) string { return chain + ":" + address }

// Seed fetches the single smart-money/dex-trades call and derives the
// unique wallet and token universes from it.
func (p *Pipeline) Seed(ctx context.Context) (*nansen.DexTradesResponse, error) {
	var resp nansen.DexTradesResponse
	if _, err := p.Client.Do(ctx, nansen.PathSmartMoneyDexTrades, SeedRequest, &resp); err != nil {
		return nil, fmt.Errorf("pipeline: seed: %w", err)
	}
	return &resp, nil
}

// deriveUniverse builds the wallet and token aggregates the seed implies.
func deriveUniverse(seed *nansen.DexTradesResponse) (map[string]*walletAgg, map[string]*tokenAgg) {
	wallets := map[string]*walletAgg{}
	tokens := map[string]*tokenAgg{}

	touchToken := func(chain, addr, symbol string) {
		if addr == "" {
			return
		}
		k := tokenKey(chain, addr)
		t, ok := tokens[k]
		if !ok {
			t = &tokenAgg{chain: chain, address: addr}
			tokens[k] = t
		}
		if t.seedSymbol == "" && symbol != "" {
			t.seedSymbol = symbol
		}
	}

	for _, tr := range seed.Data {
		ts, err := time.Parse(time.RFC3339, tr.BlockTimestamp)
		if err != nil {
			ts = time.Time{}
		}

		w, ok := wallets[tr.TraderAddress]
		if !ok {
			w = &walletAgg{address: tr.TraderAddress, chains: map[string]struct{}{}}
			wallets[tr.TraderAddress] = w
		}
		if tr.TraderAddressLabel != "" {
			w.label = tr.TraderAddressLabel
		}
		w.chains[tr.Chain] = struct{}{}
		if ts.After(w.lastTrade) {
			w.lastTrade = ts
		}
		if tr.TokenBoughtAddress != "" {
			w.boughtTokens = append(w.boughtTokens, score.WalletTokenTrade{
				TokenKey:      tokenKey(tr.Chain, tr.TokenBoughtAddress),
				TradeValueUSD: tr.TradeValueUSD,
			})
		}

		touchToken(tr.Chain, tr.TokenBoughtAddress, tr.TokenBoughtSymbol)
		touchToken(tr.Chain, tr.TokenSoldAddress, tr.TokenSoldSymbol)
	}
	return wallets, tokens
}

// SeedMaxTimestamp returns the latest block_timestamp in the seed, used
// as the deterministic reference time for scoring recency.
func SeedMaxTimestamp(seed *nansen.DexTradesResponse) time.Time {
	var max time.Time
	for _, tr := range seed.Data {
		ts, err := time.Parse(time.RFC3339, tr.BlockTimestamp)
		if err != nil {
			continue
		}
		if ts.After(max) {
			max = ts
		}
	}
	return max
}

// Result is everything the CLI needs to render the report.
type Result struct {
	Seed          *nansen.DexTradesResponse
	Wallets       []score.WalletInput
	Tokens        []score.TokenInput
	Failures      []Failure
	WalletCount   int
	TokenCount    int
	RankedWallets []score.WalletScore

	// TokenReports is the product: one falsifier verdict per token,
	// sorted most-concerning first. See docs/DESIGN.md's Output section.
	TokenReports []TokenReport
	// Census is the whole-universe funder census (not per-token), the
	// input that reproduces docs/DESIGN.md's "16 of 50 wallets trace to
	// 2 funders" headline.
	Census score.FunderCensus
}

// TokenReport is one token's falsifier verdict plus everything the report
// needs to let a reader disagree with it: the raw signed flow numbers and
// the cluster-collapse breakdown of who bought it.
type TokenReport struct {
	Key     string
	Chain   string
	Address string
	Symbol  string // never empty: falls back to the bare address
	Name    string

	Verdict score.Verdict
	N       int // distinct buying wallets
	M       int // distinct clusters after collapse

	SmartTraderNetFlowUSD float64
	ExchangeNetFlowUSD    float64
	FlowAvailable         bool
	LiquidityUSD          *float64

	Clusters []score.ClusterGroup
}

// verdictSeverity orders verdicts by how much the row actually says, per
// docs/DESIGN.md's Output section: BOTH, THIN, CONCENTRATED, then
// DISTRIBUTING and EXIT_ONLY tied (both are the exit signal firing —
// EXIT_ONLY is the same finding as DISTRIBUTING minus a known independence
// read, so it ranks alongside it rather than ahead of the differentiated
// independence findings, which are this product's distinguishing part),
// then CONFIRMED. WEAK (N < 3, no independence signal AND no exit signal)
// sorts last and is collapsed behind a disclosure line by the report
// rather than occupying rows in the main table; EXIT_ONLY, despite also
// having N < 3, is never collapsed — see docs/DESIGN.md's Output section.
func verdictSeverity(v score.Verdict) int {
	switch v {
	case score.VerdictBoth:
		return 0
	case score.VerdictThin:
		return 1
	case score.VerdictConcentrated:
		return 2
	case score.VerdictDistributing, score.VerdictExitOnly:
		return 3
	case score.VerdictConfirmed:
		return 4
	default: // WEAK
		return 5
	}
}

// Run executes the full pipeline: seed, wallet enrichment, token
// enrichment, then scoring. Partial failures during enrichment are
// collected in Result.Failures rather than aborting the run.
func (p *Pipeline) Run(ctx context.Context) (*Result, error) {
	seed, err := p.Seed(ctx)
	if err != nil {
		return nil, err
	}
	walletAggs, tokenAggs := deriveUniverse(seed)
	walletAggs, tokenAggs = limitUniverse(walletAggs, tokenAggs, p.MaxWallets)
	ref := SeedMaxTimestamp(seed)

	var mu sync.Mutex
	var failures []Failure
	recordFailure := func(stage, key string, err error) {
		mu.Lock()
		failures = append(failures, Failure{Stage: stage, Key: key, Err: err})
		mu.Unlock()
	}

	walletResults := make(map[string]*walletEnrichment, len(walletAggs))
	for addr := range walletAggs {
		walletResults[addr] = &walletEnrichment{}
	}

	// pnl-summary: one call per wallet, chain:"all" (verified working).
	p.runPool(ctx, walletKeys(walletAggs), func(ctx context.Context, addr string) {
		res := walletResults[addr]

		var pnl nansen.PnLSummaryResponse
		_, err := p.Client.Do(ctx, nansen.PathProfilerPnLSummary, nansen.PnLSummaryRequest{
			Address: addr,
			Chain:   "all",
			Date:    nansen.DateRangeReq{From: p.DateFrom, To: p.DateTo},
		}, &pnl)
		if err != nil {
			recordFailure("wallet:pnl-summary", addr, err)
		} else {
			res.pnl = &pnl
		}
	})

	// related-wallets: chain:"all" is rejected (422 invalid_field_value),
	// so this is one call per (wallet, chain) pair actually seen in the
	// seed — 187 pairs across 172 wallets. Concurrent calls for the same
	// wallet on different chains write into the same *walletEnrichment,
	// hence its own mutex.
	relatedItems := make([]string, 0, 187)
	for addr, w := range walletAggs {
		for chain := range w.chains {
			relatedItems = append(relatedItems, walletChainKey(addr, chain))
		}
	}
	sort.Strings(relatedItems)
	p.runPool(ctx, relatedItems, func(ctx context.Context, item string) {
		addr, chain, _ := splitWalletChainKey(item)
		res := walletResults[addr]

		var related nansen.RelatedWalletsResponse
		_, err := p.Client.Do(ctx, nansen.PathProfilerRelatedWallets, nansen.RelatedWalletsRequest{
			Address: addr,
			Chain:   chain,
		}, &related)
		if err != nil {
			recordFailure("wallet:related-wallets", item, err)
			return
		}
		res.mu.Lock()
		res.relatedWallets = append(res.relatedWallets, related.Data...)
		res.mu.Unlock()
	})

	tokenResults := make(map[string]*tokenEnrichment, len(tokenAggs))
	for k := range tokenAggs {
		tokenResults[k] = &tokenEnrichment{}
	}
	p.runPool(ctx, tokenKeys(tokenAggs), func(ctx context.Context, k string) {
		agg := tokenAggs[k]
		res := tokenResults[k]

		var flow nansen.FlowIntelligenceResponse
		_, err := p.Client.Do(ctx, nansen.PathTGMFlowIntelligence, nansen.FlowIntelligenceRequest{
			Chain:        agg.chain,
			TokenAddress: agg.address,
			Timeframe:    "1d",
		}, &flow)
		if err != nil {
			recordFailure("token:flow-intelligence", k, err)
		} else {
			res.flow = &flow
		}

		var info nansen.TokenInformationResponse
		_, err = p.Client.Do(ctx, nansen.PathTGMTokenInformation, nansen.TokenInformationRequest{
			Chain:        agg.chain,
			TokenAddress: agg.address,
			Timeframe:    "1d",
		}, &info)
		if err != nil {
			recordFailure("token:token-information", k, err)
		} else {
			res.info = &info
		}
	})

	tokens := make([]score.TokenInput, 0, len(tokenAggs))
	tokenMeta := make(map[string]score.TokenInput, len(tokenAggs))
	for k, agg := range tokenAggs {
		ti := score.TokenInput{Key: k, Symbol: agg.seedSymbol}
		if r := tokenResults[k]; r != nil && r.flow != nil && len(r.flow.Data) > 0 {
			fi := r.flow.Data[0]
			ti.SmartTraderNetFlowUSD = fi.SmartTraderNetFlowUSD
			ti.ExchangeNetFlowUSD = fi.ExchangeNetFlowUSD
			ti.FlowAvailable = true
		}
		if r := tokenResults[k]; r != nil && r.info != nil {
			// tgm/token-information carries the real name and symbol; the
			// seed's own symbol is only a fallback for offline gaps.
			if r.info.Data.Symbol != "" {
				ti.Symbol = r.info.Data.Symbol
			}
			ti.Name = r.info.Data.Name
			ti.LiquidityUSD = r.info.Data.SpotMetrics.LiquidityUSD
		}
		tokens = append(tokens, ti)
		tokenMeta[k] = ti
	}

	// walletFunders holds each wallet's First-Funder edges only —
	// docs/DESIGN.md: every other relation string (Deployed Program,
	// Deployed Contract, Deployed via, Deployed by, Created by, and any
	// future/unknown relation) is a contract artefact, never a funding
	// edge, and must be filtered out here before it reaches score.Cluster.
	walletFunders := make(map[string][]score.Funder, len(walletAggs))
	for addr := range walletAggs {
		r := walletResults[addr]
		if r == nil {
			continue
		}
		var funders []score.Funder
		seen := map[string]bool{}
		for _, rel := range r.relatedWallets {
			if rel.Relation != "First Funder" {
				continue
			}
			if seen[rel.Address] {
				continue
			}
			seen[rel.Address] = true
			funders = append(funders, score.Funder{Address: rel.Address, Label: rel.AddressLabel})
		}
		if len(funders) > 0 {
			walletFunders[addr] = funders
		}
	}

	wallets := make([]score.WalletInput, 0, len(walletAggs))
	for addr, agg := range walletAggs {
		wi := score.WalletInput{
			Address:       addr,
			Label:         agg.label,
			LastTradeTime: agg.lastTrade,
			BoughtTokens:  agg.boughtTokens,
			Funders:       walletFunders[addr],
		}
		if r := walletResults[addr]; r != nil && r.pnl != nil {
			wi.RealizedPnLUSD = r.pnl.RealizedPnLUSD
			wi.WinRate = r.pnl.WinRate
		}
		wallets = append(wallets, wi)
	}

	rankedWallets := score.ScoreWallets(wallets, tokens, ref)

	walletByAddr := make(map[string]score.WalletInput, len(wallets))
	for _, w := range wallets {
		walletByAddr[w.Address] = w
	}

	// tokenBuyers: reverse index from token key to the (limited) wallets
	// that bought it, the seed input for each token's cluster collapse.
	tokenBuyers := map[string][]string{}
	for addr, agg := range walletAggs {
		seenTok := map[string]bool{}
		for _, bt := range agg.boughtTokens {
			if seenTok[bt.TokenKey] {
				continue
			}
			seenTok[bt.TokenKey] = true
			tokenBuyers[bt.TokenKey] = append(tokenBuyers[bt.TokenKey], addr)
		}
	}

	tokenReports := make([]TokenReport, 0, len(tokenAggs))
	for k, agg := range tokenAggs {
		buyerAddrs := tokenBuyers[k]
		sort.Strings(buyerAddrs)
		buyers := make([]score.BuyerInput, 0, len(buyerAddrs))
		for _, addr := range buyerAddrs {
			w := walletByAddr[addr]
			buyers = append(buyers, score.BuyerInput{
				Address: w.Address,
				Label:   w.Label,
				PnLUSD:  w.RealizedPnLUSD,
				WinRate: w.WinRate,
				Funders: w.Funders,
			})
		}
		cr := score.Cluster(buyers)
		ti := tokenMeta[k]
		verdict := score.DecideVerdict(cr.N, cr.M, ti.SmartTraderNetFlowUSD, ti.ExchangeNetFlowUSD, ti.FlowAvailable)

		symbol := ti.Symbol
		if symbol == "" {
			symbol = agg.address
		}
		tokenReports = append(tokenReports, TokenReport{
			Key:                   k,
			Chain:                 agg.chain,
			Address:               agg.address,
			Symbol:                symbol,
			Name:                  ti.Name,
			Verdict:               verdict,
			N:                     cr.N,
			M:                     cr.M,
			SmartTraderNetFlowUSD: ti.SmartTraderNetFlowUSD,
			ExchangeNetFlowUSD:    ti.ExchangeNetFlowUSD,
			FlowAvailable:         ti.FlowAvailable,
			LiquidityUSD:          ti.LiquidityUSD,
			Clusters:              cr.Clusters,
		})
	}
	sort.Slice(tokenReports, func(i, j int) bool {
		si, sj := verdictSeverity(tokenReports[i].Verdict), verdictSeverity(tokenReports[j].Verdict)
		if si != sj {
			return si < sj
		}
		if tokenReports[i].N != tokenReports[j].N {
			return tokenReports[i].N > tokenReports[j].N
		}
		return tokenReports[i].Key < tokenReports[j].Key
	})

	censusInput := make([]score.BuyerInput, 0, len(wallets))
	for _, w := range wallets {
		censusInput = append(censusInput, score.BuyerInput{Address: w.Address, Label: w.Label, Funders: w.Funders})
	}
	census := score.ComputeFunderCensus(censusInput)

	sort.Slice(failures, func(i, j int) bool {
		if failures[i].Stage != failures[j].Stage {
			return failures[i].Stage < failures[j].Stage
		}
		return failures[i].Key < failures[j].Key
	})

	return &Result{
		Seed:          seed,
		Wallets:       wallets,
		Tokens:        tokens,
		Failures:      failures,
		WalletCount:   len(walletAggs),
		TokenCount:    len(tokenAggs),
		RankedWallets: rankedWallets,
		TokenReports:  tokenReports,
		Census:        census,
	}, nil
}

type walletEnrichment struct {
	pnl *nansen.PnLSummaryResponse

	mu             sync.Mutex
	relatedWallets []nansen.RelatedWallet
}

type tokenEnrichment struct {
	flow *nansen.FlowIntelligenceResponse
	info *nansen.TokenInformationResponse
}

func walletKeys(m map[string]*walletAgg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func tokenKeys(m map[string]*tokenAgg) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// runPool runs fn(ctx, item) for each item using p.Concurrency workers.
// Context cancellation stops dispatch of new work and is propagated into
// fn via ctx; in-flight goroutines observe ctx.Done() through the client's
// own context-aware HTTP calls and rate limiter waits.
// runPool runs fn for each item, but stays serialized (one in flight)
// until the client has observed a real X-Nansen-Credits-Remaining from a
// live response — so at most one call is ever "blind" to the budget
// floor. Cache hits don't establish that balance, so several leading
// cached items may be processed serially before the first live call
// teaches us it, at which point the rest fan out at full concurrency.
func (p *Pipeline) runPool(ctx context.Context, items []string, fn func(ctx context.Context, item string)) {
	idx := 0
	for idx < len(items) && p.Client.CreditsRemaining() < 0 {
		if ctx.Err() != nil {
			return
		}
		fn(ctx, items[idx])
		idx++
	}
	remaining := items[idx:]

	sem := make(chan struct{}, p.Concurrency)
	var wg sync.WaitGroup
dispatch:
	for _, item := range remaining {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			break dispatch
		}
		wg.Add(1)
		go func(item string) {
			defer wg.Done()
			defer func() { <-sem }()
			fn(ctx, item)
		}(item)
	}
	wg.Wait()
}
