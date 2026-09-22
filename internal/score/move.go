package score

// Move is how far a token has travelled since smart money bought it.
//
// A verdict answers "is this signal real?", but not "is it still early?". X7
// came out CONFIRMED correctly — independent buyers, no exchange inflow — yet
// by the time the seed was fetched it had already run 30x past the smart-money
// entry and was on its way down. A reader acting on the verdict alone would
// have bought near the top. Move puts that timing next to the verdict.
type Move struct {
	// EntryPrice is the volume-weighted price smart money paid in the seed.
	EntryPrice float64
	// PeakMultiple is the highest high since entry, over EntryPrice.
	PeakMultiple float64
	// NowMultiple is the latest close over EntryPrice.
	NowMultiple float64
	// FromPeak is how far the latest close sits below the peak, 0..1.
	FromPeak float64
}

// Buy is one smart-money purchase: what was spent and what it bought.
type Buy struct {
	ValueUSD float64
	Amount   float64
}

// Candle is the part of an OHLCV bar the move needs. High and Close are
// pointers because the API returns null for bars with no trades.
type Candle struct {
	High  *float64
	Close *float64
}

// EntryPrice is the volume-weighted average price of the buys, or false when
// there is nothing to average (no buys, or buys with no token amount).
func EntryPrice(buys []Buy) (float64, bool) {
	var usd, amount float64
	for _, b := range buys {
		if b.ValueUSD <= 0 || b.Amount <= 0 {
			continue
		}
		usd += b.ValueUSD
		amount += b.Amount
	}
	if amount == 0 {
		return 0, false
	}
	return usd / amount, true
}

// ComputeMove measures candles, oldest first, against the smart-money entry.
// It returns false rather than a guess when either side is missing: a move
// computed from absent data would put a confident multiple on screen for a
// token nobody priced.
func ComputeMove(buys []Buy, candles []Candle) (Move, bool) {
	entry, ok := EntryPrice(buys)
	if !ok {
		return Move{}, false
	}
	// Smart money's own fills are trades that happened, so the peak is at
	// least the highest of them. The hourly bar can miss them: REKT's buys
	// printed at up to $0.00019 while its bar topped out at $0.000106,
	// because the token collapsed 98% within minutes of the buys.
	var peak, last float64
	for _, b := range buys {
		if b.ValueUSD > 0 && b.Amount > 0 && b.ValueUSD/b.Amount > peak {
			peak = b.ValueUSD / b.Amount
		}
	}
	candlePeak := false
	for _, c := range candles {
		if c.High != nil && *c.High > 0 {
			candlePeak = true
			if *c.High > peak {
				peak = *c.High
			}
		}
		if c.Close != nil && *c.Close > 0 {
			last = *c.Close
		}
	}
	if !candlePeak || last == 0 {
		return Move{}, false
	}
	m := Move{
		EntryPrice:   entry,
		PeakMultiple: peak / entry,
		NowMultiple:  last / entry,
	}
	m.FromPeak = 1 - last/peak
	return m, true
}
