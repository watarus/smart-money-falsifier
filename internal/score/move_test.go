package score

import (
	"math"
	"testing"
)

func p(v float64) *float64 { return &v }

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9*math.Max(1, math.Abs(b)) }

func TestEntryPrice(t *testing.T) {
	tests := []struct {
		name string
		buys []Buy
		want float64
		ok   bool
	}{
		{"volume weighted, not a plain average", []Buy{{100, 100}, {300, 100}}, 2, true},
		{"zero-amount rows are skipped", []Buy{{100, 50}, {80, 0}}, 2, true},
		{"no buys", nil, 0, false},
		{"only unusable rows", []Buy{{0, 10}, {10, 0}}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := EntryPrice(tt.buys)
			if ok != tt.ok || (ok && !near(got, tt.want)) {
				t.Fatalf("EntryPrice = (%v, %v), want (%v, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

// X7 in the real data: smart money paid ~$0.0000973, the token peaked at
// $0.00295 and last closed at $0.000347.
func TestComputeMove_RunUpAndFade(t *testing.T) {
	buys := []Buy{{4750, 4750 / 9.727125281102866e-05}}
	candles := []Candle{
		{High: p(0.00018), Close: p(0.00018)},
		{High: p(0.0029501136266877957), Close: p(0.0024)},
		{High: nil, Close: nil}, // a bar with no trades
		{High: p(0.0006), Close: p(0.0003472291240499553)},
	}
	m, ok := ComputeMove(buys, candles)
	if !ok {
		t.Fatal("expected a move")
	}
	if math.Abs(m.PeakMultiple-30.33) > 0.01 {
		t.Errorf("PeakMultiple = %.2f, want ~30.33", m.PeakMultiple)
	}
	if math.Abs(m.NowMultiple-3.57) > 0.01 {
		t.Errorf("NowMultiple = %.2f, want ~3.57", m.NowMultiple)
	}
	if math.Abs(m.FromPeak-0.882) > 0.001 {
		t.Errorf("FromPeak = %.3f, want ~0.882", m.FromPeak)
	}
}

func TestComputeMove_MissingDataIsNotAMove(t *testing.T) {
	buys := []Buy{{100, 100}}
	tests := []struct {
		name    string
		buys    []Buy
		candles []Candle
	}{
		{"no candles", buys, nil},
		{"only empty bars", buys, []Candle{{}, {}}},
		{"no entry price", nil, []Candle{{High: p(2), Close: p(2)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if m, ok := ComputeMove(tt.buys, tt.candles); ok {
				t.Fatalf("expected no move from missing data, got %+v", m)
			}
		})
	}
}

// The latest close is the last bar that traded, not the last bar returned.
func TestComputeMove_TrailingEmptyBarIgnored(t *testing.T) {
	m, ok := ComputeMove([]Buy{{100, 100}}, []Candle{
		{High: p(3), Close: p(2)},
		{High: nil, Close: nil},
	})
	if !ok || !near(m.NowMultiple, 2) {
		t.Fatalf("got %+v ok=%v, want NowMultiple 2", m, ok)
	}
}

// REKT in the real data: smart money paid up to $0.00019 at 05:47 UTC, the
// token collapsed 98% within minutes, and the hourly bar's high ($0.000106)
// never shows the prices they paid. A peak below the entry would claim the
// token never traded where smart money bought it.
func TestComputeMove_PeakNeverBelowSmartMoneyFills(t *testing.T) {
	buys := []Buy{{273.34, 1441745.657}, {41.72, 542352.907}}
	m, ok := ComputeMove(buys, []Candle{
		{High: p(0.00010595), Close: p(0.0000027)},
		{High: p(0.0000027), Close: p(0.0000025)},
	})
	if !ok {
		t.Fatal("expected a move")
	}
	if m.PeakMultiple < 1 {
		t.Fatalf("PeakMultiple = %.2f; the peak can't sit below prices smart money actually paid", m.PeakMultiple)
	}
	if m.NowMultiple > 0.05 {
		t.Fatalf("NowMultiple = %.3f, want the collapse to show (<0.05)", m.NowMultiple)
	}
}
