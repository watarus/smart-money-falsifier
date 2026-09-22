package nansen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// fixtureDir locates data/fixtures relative to the repo root, which is two
// levels up from this package.
func fixtureDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "data", "fixtures"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("fixtures dir not available: %v", err)
	}
	return dir
}

func readFixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

func TestDecodeDexTradesFixture(t *testing.T) {
	dir := fixtureDir(t)
	raw := readFixture(t, dir, "smart-money_dex-trades.json")

	var resp DexTradesResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 1000 {
		t.Fatalf("expected 1000 trades, got %d", len(resp.Data))
	}

	wallets := map[string]struct{}{}
	tokens := map[string]struct{}{}
	for _, tr := range resp.Data {
		if tr.TraderAddress == "" {
			t.Fatalf("trade missing trader_address: %+v", tr)
		}
		wallets[tr.TraderAddress] = struct{}{}
		if tr.TokenBoughtAddress != "" {
			tokens[tr.TokenBoughtAddress] = struct{}{}
		}
		if tr.TokenSoldAddress != "" {
			tokens[tr.TokenSoldAddress] = struct{}{}
		}
	}
	if len(wallets) != 172 {
		t.Fatalf("expected 172 unique wallets, got %d", len(wallets))
	}
	if len(tokens) == 0 {
		t.Fatalf("expected non-zero unique tokens")
	}
}

func TestDecodePnLSummaryFixture(t *testing.T) {
	dir := fixtureDir(t)
	raw := readFixture(t, dir, "profiler_address_pnl-summary.json")

	var resp PnLSummaryResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.TradedTokenCount == 0 {
		t.Fatalf("expected non-zero traded_token_count")
	}
	if resp.TradedTimes == 0 {
		t.Fatalf("expected non-zero traded_times")
	}
	if len(resp.Top5Tokens) != 5 {
		t.Fatalf("expected 5 top5_tokens, got %d", len(resp.Top5Tokens))
	}
}

func TestDecodeRelatedWalletsFixture(t *testing.T) {
	dir := fixtureDir(t)
	raw := readFixture(t, dir, "profiler_address_related-wallets.json")

	var resp RelatedWalletsResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected at least 1 related wallet")
	}
	if resp.Data[0].Address == "" {
		t.Fatalf("related wallet missing address")
	}
}

func TestDecodeTokenInformationFixture(t *testing.T) {
	dir := fixtureDir(t)
	raw := readFixture(t, dir, "tgm_token-information.json")

	var resp TokenInformationResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Data.Symbol == "" {
		t.Fatalf("expected non-empty symbol")
	}
	if resp.Data.SpotMetrics.LiquidityUSD == nil || *resp.Data.SpotMetrics.LiquidityUSD == 0 {
		t.Fatalf("expected non-zero liquidity_usd, got %+v", resp.Data.SpotMetrics.LiquidityUSD)
	}
	if resp.Data.TokenDetails.MarketCapUSD == nil || *resp.Data.TokenDetails.MarketCapUSD == 0 {
		t.Fatalf("expected non-zero market_cap_usd, got %+v", resp.Data.TokenDetails.MarketCapUSD)
	}
}

func TestDecodeFlowIntelligenceFixture(t *testing.T) {
	dir := fixtureDir(t)
	raw := readFixture(t, dir, "tgm_flow-intelligence.json")

	var resp FlowIntelligenceResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) == 0 {
		t.Fatalf("expected at least 1 flow-intelligence row")
	}
	if resp.Data[0].SmartTraderWalletCount == 0 {
		t.Fatalf("expected non-zero smart_trader_wallet_count")
	}
}
