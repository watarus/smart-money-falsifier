package pipeline

import (
	"context"
	"fmt"

	"github.com/watarus/nansen/internal/nansen"
)

// PlanItem describes one stage of the call plan: how many calls it needs,
// how many are already cached, and the resulting credit estimate.
type PlanItem struct {
	Endpoint    string
	Total       int
	Cached      int
	New         int
	CostPerCall int
	// UpperBound marks a stage whose true size depends on results the plan
	// cannot compute without HTTP. Its Total is the most it can call, so the
	// credit estimate errs high rather than surprising the operator.
	UpperBound bool
}

func (i PlanItem) NewCreditsEstimate() int { return i.New * i.CostPerCall }

// Plan computes the exact call plan without making any HTTP request. The
// seed itself must already be resolvable from cache (a live call, or the
// data/fixtures import via ImportSeedFixture) for wallet/token counts to
// be known; if it isn't, only the seed line of the plan is returned.
func (p *Pipeline) Plan(ctx context.Context) ([]PlanItem, error) {
	seedPlan, err := p.Client.PlanCall(nansen.PathSmartMoneyDexTrades, SeedRequest)
	if err != nil {
		return nil, err
	}
	items := []PlanItem{
		planItemFromCall(nansen.PathSmartMoneyDexTrades, 1, seedPlan.CacheHit, seedPlan.CreditsCost),
	}

	if !seedPlan.CacheHit {
		// Can't know wallet/token counts without the seed; report what we
		// can and stop here (dry-run must never make an HTTP call to find
		// out more).
		return items, nil
	}

	var seed nansen.DexTradesResponse
	if _, err := p.Client.Do(ctx, nansen.PathSmartMoneyDexTrades, SeedRequest, &seed); err != nil {
		return nil, fmt.Errorf("pipeline: plan: reading cached seed: %w", err)
	}
	walletAggs, tokenAggs := deriveUniverse(&seed)
	walletAggs, tokenAggs = limitUniverse(walletAggs, tokenAggs, p.MaxWallets)

	// pnl-summary: one call per wallet, chain:"all" (verified working).
	pnlItem := func() (PlanItem, error) {
		cached, total := 0, 0
		for addr := range walletAggs {
			total++
			body := nansen.PnLSummaryRequest{Address: addr, Chain: "all", Date: nansen.DateRangeReq{From: p.DateFrom, To: p.DateTo}}
			plan, err := p.Client.PlanCall(nansen.PathProfilerPnLSummary, body)
			if err != nil {
				return PlanItem{}, err
			}
			if plan.CacheHit {
				cached++
			}
		}
		return planItemFromCall(nansen.PathProfilerPnLSummary, total, false, nansen.CreditCost[nansen.PathProfilerPnLSummary]).withCached(cached), nil
	}

	// related-wallets: chain:"all" is rejected (422 invalid_field_value,
	// verified live), so this is one call per (wallet, chain) pair
	// actually present in the seed.
	relatedItem := func() (PlanItem, error) {
		cached, total := 0, 0
		for addr, w := range walletAggs {
			for chain := range w.chains {
				total++
				body := nansen.RelatedWalletsRequest{Address: addr, Chain: chain}
				plan, err := p.Client.PlanCall(nansen.PathProfilerRelatedWallets, body)
				if err != nil {
					return PlanItem{}, err
				}
				if plan.CacheHit {
					cached++
				}
			}
		}
		return planItemFromCall(nansen.PathProfilerRelatedWallets, total, false, nansen.CreditCost[nansen.PathProfilerRelatedWallets]).withCached(cached), nil
	}

	tokenItem := func(path string) (PlanItem, error) {
		cached, total := 0, 0
		for _, agg := range tokenAggs {
			total++
			var body any
			switch path {
			case nansen.PathTGMFlowIntelligence:
				body = nansen.FlowIntelligenceRequest{Chain: agg.chain, TokenAddress: agg.address, Timeframe: "1d"}
			case nansen.PathTGMTokenInformation:
				body = nansen.TokenInformationRequest{Chain: agg.chain, TokenAddress: agg.address, Timeframe: "1d"}
			}
			plan, err := p.Client.PlanCall(path, body)
			if err != nil {
				return PlanItem{}, err
			}
			if plan.CacheHit {
				cached++
			}
		}
		return planItemFromCall(path, total, false, nansen.CreditCost[path]).withCached(cached), nil
	}

	pnl, err := pnlItem()
	if err != nil {
		return nil, err
	}
	items = append(items, pnl)
	related, err := relatedItem()
	if err != nil {
		return nil, err
	}
	items = append(items, related)

	for _, path := range []string{nansen.PathTGMFlowIntelligence, nansen.PathTGMTokenInformation} {
		item, err := tokenItem(path)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	// Only tokens that end up with a verdict are priced, and verdicts need the
	// enrichment above. Plan every token smart money bought instead: that is
	// the ceiling.
	buys := seedBuys(&seed)
	ohlcv := PlanItem{Endpoint: nansen.PathTGMTokenOHLCV, CostPerCall: nansen.CreditCost[nansen.PathTGMTokenOHLCV], UpperBound: true}
	for k, agg := range tokenAggs {
		if len(buys[k]) == 0 {
			continue
		}
		ohlcv.Total++
		plan, err := p.Client.PlanCall(nansen.PathTGMTokenOHLCV, moveRequest(agg.chain, agg.address))
		if err != nil {
			return nil, err
		}
		if plan.CacheHit {
			ohlcv.Cached++
		}
	}
	ohlcv.New = ohlcv.Total - ohlcv.Cached
	items = append(items, ohlcv)
	return items, nil
}

func planItemFromCall(endpoint string, total int, cacheHit bool, cost int) PlanItem {
	cached := 0
	if cacheHit {
		cached = 1
	}
	return PlanItem{Endpoint: endpoint, Total: total, Cached: cached, New: total - cached, CostPerCall: cost}
}

func (i PlanItem) withCached(cached int) PlanItem {
	i.Cached = cached
	i.New = i.Total - cached
	return i
}
