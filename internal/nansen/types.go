// Package nansen provides a budget-aware, cached, rate-limited HTTP client
// for the Nansen API, plus typed request/response structs for the five
// endpoints used by the scorecard pipeline. See docs/DESIGN.md.
package nansen

// Endpoint paths (relative to https://api.nansen.ai/api/v1/).
const (
	PathSmartMoneyDexTrades    = "smart-money/dex-trades"
	PathProfilerPnLSummary     = "profiler/address/pnl-summary"
	PathProfilerRelatedWallets = "profiler/address/related-wallets"
	PathTGMFlowIntelligence    = "tgm/flow-intelligence"
	PathTGMTokenInformation    = "tgm/token-information"
)

// CreditCost is the declared credit cost per call for each endpoint, per
// docs/DESIGN.md. smart-money/* costs 5; everything else costs 1.
var CreditCost = map[string]int{
	PathSmartMoneyDexTrades:    5,
	PathProfilerPnLSummary:     1,
	PathProfilerRelatedWallets: 1,
	PathTGMFlowIntelligence:    1,
	PathTGMTokenInformation:    1,
	PathTGMTokenOHLCV:          1,
}

// --- smart-money/dex-trades ---

type DexTradesRequest struct {
	Chains     []string      `json:"chains"`
	Pagination PaginationReq `json:"pagination"`
}

type PaginationReq struct {
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

type DexTradesResponse struct {
	Data       []DexTrade     `json:"data"`
	Pagination PaginationResp `json:"pagination"`
}

type PaginationResp struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	IsLastPage bool `json:"is_last_page"`
}

type DexTrade struct {
	Chain                string   `json:"chain"`
	BlockTimestamp       string   `json:"block_timestamp"`
	TransactionHash      string   `json:"transaction_hash"`
	TraderAddress        string   `json:"trader_address"`
	TraderAddressLabel   string   `json:"trader_address_label"`
	TokenBoughtAddress   string   `json:"token_bought_address"`
	TokenSoldAddress     string   `json:"token_sold_address"`
	TokenBoughtAmount    float64  `json:"token_bought_amount"`
	TokenSoldAmount      float64  `json:"token_sold_amount"`
	TokenBoughtSymbol    string   `json:"token_bought_symbol"`
	TokenSoldSymbol      string   `json:"token_sold_symbol"`
	TokenBoughtAgeDays   *int     `json:"token_bought_age_days"`
	TokenSoldAgeDays     *int     `json:"token_sold_age_days"`
	TokenBoughtMarketCap *float64 `json:"token_bought_market_cap"`
	TokenSoldMarketCap   *float64 `json:"token_sold_market_cap"`
	TokenBoughtFDV       *float64 `json:"token_bought_fdv"`
	TokenSoldFDV         *float64 `json:"token_sold_fdv"`
	TradeValueUSD        float64  `json:"trade_value_usd"`
}

// --- profiler/address/pnl-summary ---

type PnLSummaryRequest struct {
	Address string       `json:"address"`
	Chain   string       `json:"chain"`
	Date    DateRangeReq `json:"date"`
}

type DateRangeReq struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type PnLSummaryResponse struct {
	Pagination         PaginationResp `json:"pagination"`
	Top5Tokens         []Top5Token    `json:"top5_tokens"`
	TradedTokenCount   int            `json:"traded_token_count"`
	TradedTimes        int            `json:"traded_times"`
	RealizedPnLUSD     float64        `json:"realized_pnl_usd"`
	RealizedPnLPercent float64        `json:"realized_pnl_percent"`
	WinRate            float64        `json:"win_rate"`
}

type Top5Token struct {
	RealizedPnL  float64 `json:"realized_pnl"`
	RealizedROI  float64 `json:"realized_roi"`
	TokenAddress string  `json:"token_address"`
	TokenSymbol  string  `json:"token_symbol"`
	Chain        string  `json:"chain"`
}

// --- profiler/address/related-wallets ---

type RelatedWalletsRequest struct {
	Address string `json:"address"`
	Chain   string `json:"chain"`
}

type RelatedWalletsResponse struct {
	Pagination PaginationResp  `json:"pagination"`
	Data       []RelatedWallet `json:"data"`
}

type RelatedWallet struct {
	Address         string `json:"address"`
	AddressLabel    string `json:"address_label"`
	Relation        string `json:"relation"`
	TransactionHash string `json:"transaction_hash"`
	BlockTimestamp  string `json:"block_timestamp"`
	Order           int    `json:"order"`
	Chain           string `json:"chain"`
}

// --- tgm/flow-intelligence ---

type FlowIntelligenceRequest struct {
	Chain        string `json:"chain"`
	TokenAddress string `json:"token_address"`
	Timeframe    string `json:"timeframe"`
}

type FlowIntelligenceResponse struct {
	Data     []FlowIntelligence `json:"data"`
	Warnings []string           `json:"warnings"`
}

type FlowIntelligence struct {
	PublicFigureNetFlowUSD  float64 `json:"public_figure_net_flow_usd"`
	PublicFigureAvgFlowUSD  float64 `json:"public_figure_avg_flow_usd"`
	PublicFigureWalletCount int     `json:"public_figure_wallet_count"`
	TopPnLNetFlowUSD        float64 `json:"top_pnl_net_flow_usd"`
	TopPnLAvgFlowUSD        float64 `json:"top_pnl_avg_flow_usd"`
	TopPnLWalletCount       int     `json:"top_pnl_wallet_count"`
	WhaleNetFlowUSD         float64 `json:"whale_net_flow_usd"`
	WhaleAvgFlowUSD         float64 `json:"whale_avg_flow_usd"`
	WhaleWalletCount        int     `json:"whale_wallet_count"`
	SmartTraderNetFlowUSD   float64 `json:"smart_trader_net_flow_usd"`
	SmartTraderAvgFlowUSD   float64 `json:"smart_trader_avg_flow_usd"`
	SmartTraderWalletCount  int     `json:"smart_trader_wallet_count"`
	ExchangeNetFlowUSD      float64 `json:"exchange_net_flow_usd"`
	// ExchangeAvgFlowUSD is a pointer because null is the only field that
	// separates "no exchange address touched this token" from "exchanges
	// saw flow that happened to net to zero": the net flow reads 0.0 in both
	// cases, and exchange_wallet_count reads 0 even when there was flow.
	// Across the full seed it is null on 129 of 330 tokens.
	ExchangeAvgFlowUSD      *float64 `json:"exchange_avg_flow_usd"`
	ExchangeWalletCount     int      `json:"exchange_wallet_count"`
	FreshWalletsNetFlowUSD  float64  `json:"fresh_wallets_net_flow_usd"`
	FreshWalletsAvgFlowUSD  float64  `json:"fresh_wallets_avg_flow_usd"`
	FreshWalletsWalletCount int      `json:"fresh_wallets_wallet_count"`
}

// --- tgm/token-information ---
// Verified against data/fixtures/tgm_token-information.json (a real 200
// response).

type TokenInformationRequest struct {
	Chain        string `json:"chain"`
	TokenAddress string `json:"token_address"`
	Timeframe    string `json:"timeframe"`
}

type TokenInformationResponse struct {
	Data TokenInformation `json:"data"`
}

type TokenInformation struct {
	Name            string           `json:"name"`
	Symbol          string           `json:"symbol"`
	ContractAddress string           `json:"contract_address"`
	Logo            string           `json:"logo"`
	TokenDetails    TokenDetails     `json:"token_details"`
	SpotMetrics     TokenSpotMetrics `json:"spot_metrics"`
}

type TokenDetails struct {
	TokenDeploymentDate string   `json:"token_deployment_date"`
	Website             string   `json:"website"`
	X                   string   `json:"x"`
	Telegram            string   `json:"telegram"`
	MarketCapUSD        *float64 `json:"market_cap_usd"`
	FDVUSD              *float64 `json:"fdv_usd"`
	CirculatingSupply   *float64 `json:"circulating_supply"`
	TotalSupply         *float64 `json:"total_supply"`
}

type TokenSpotMetrics struct {
	VolumeTotalUSD *float64 `json:"volume_total_usd"`
	BuyVolumeUSD   *float64 `json:"buy_volume_usd"`
	SellVolumeUSD  *float64 `json:"sell_volume_usd"`
	TotalBuys      *int     `json:"total_buys"`
	TotalSells     *int     `json:"total_sells"`
	UniqueBuyers   *int     `json:"unique_buyers"`
	UniqueSellers  *int     `json:"unique_sellers"`
	LiquidityUSD   *float64 `json:"liquidity_usd"`
	TotalHolders   *int     `json:"total_holders"`
}

// --- tgm/token-ohlcv ---
// Shape verified against a live response for X7 (solana): prices are plain
// numbers, "open" is null on the first candle, and market_cap is a nested
// object rather than a number.

const PathTGMTokenOHLCV = "tgm/token-ohlcv"

type TokenOHLCVRequest struct {
	Chain        string       `json:"chain"`
	TokenAddress string       `json:"token_address"`
	Timeframe    string       `json:"timeframe"`
	Date         DateRangeReq `json:"date"`
}

type TokenOHLCVResponse struct {
	Data []OHLCVCandle `json:"data"`
}

type OHLCVCandle struct {
	IntervalStart string   `json:"interval_start"`
	Open          *float64 `json:"open"`
	High          *float64 `json:"high"`
	Low           *float64 `json:"low"`
	Close         *float64 `json:"close"`
	VolumeUSD     *float64 `json:"volume_usd"`
}
