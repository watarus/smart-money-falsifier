package score

import "testing"

func TestDecideVerdict_BelowFloorIsWeak(t *testing.T) {
	// N < 3 carries no independence signal at all, regardless of M — but
	// with the exit signal not firing here, there's no signal of any kind.
	got := DecideVerdict(2, 1, 1, -100, -100, true)
	if got != VerdictWeak {
		t.Fatalf("got %v want WEAK", got)
	}
}

func TestDecideVerdict_BelowFloorExitOnly(t *testing.T) {
	// The buyer floor gates the independence verdict only, never the exit
	// signal: N < 3 with the exit signal firing is EXIT_ONLY, not WEAK.
	got := DecideVerdict(2, 1, 1, 100, 100, true)
	if got != VerdictExitOnly {
		t.Fatalf("got %v want EXIT_ONLY", got)
	}
}

func TestDecideVerdict_Thin(t *testing.T) {
	got := DecideVerdict(5, 1, 5, -100, -100, true)
	if got != VerdictThin {
		t.Fatalf("got %v want THIN", got)
	}
}

func TestDecideVerdict_Concentrated(t *testing.T) {
	// N=4, M=2: more than one actor, but not every buyer independent.
	got := DecideVerdict(4, 2, 4, -100, -100, true)
	if got != VerdictConcentrated {
		t.Fatalf("got %v want CONCENTRATED", got)
	}
}

func TestDecideVerdict_Confirmed(t *testing.T) {
	// M == N, full coverage: no two buyers share a funder, and every
	// buyer's funder data was actually checked.
	got := DecideVerdict(5, 5, 5, -100, -100, true)
	if got != VerdictConfirmed {
		t.Fatalf("got %v want CONFIRMED", got)
	}
}

func TestDecideVerdict_Both(t *testing.T) {
	// THIN (M==1) plus the exit signal firing becomes BOTH.
	got := DecideVerdict(3, 1, 3, 100, 100, true)
	if got != VerdictBoth {
		t.Fatalf("got %v want BOTH", got)
	}
}

func TestDecideVerdict_BothFromConcentrated(t *testing.T) {
	// CONCENTRATED (1<M<N) plus the exit signal firing also becomes BOTH.
	got := DecideVerdict(4, 2, 4, 100, 100, true)
	if got != VerdictBoth {
		t.Fatalf("got %v want BOTH", got)
	}
}

func TestDecideVerdict_Distributing(t *testing.T) {
	// CONFIRMED (M==N, full coverage) plus the exit signal firing becomes
	// DISTRIBUTING.
	got := DecideVerdict(4, 4, 4, 100, 100, true)
	if got != VerdictDistributing {
		t.Fatalf("got %v want DISTRIBUTING", got)
	}
}

func TestDecideVerdict_MissingFlowNeverFiresDistribution(t *testing.T) {
	// Even though the signed values look like a distribution firing, no
	// flow data means the signal must not fire — and must not be cleared
	// either. This test used to expect CONFIRMED, which is the bug: a token
	// no exchange ever touched passed an exit check that never ran.
	got := DecideVerdict(4, 4, 4, 100, 100, false)
	if got != VerdictIndependent {
		t.Fatalf("missing exchange data must yield INDEPENDENT, never CONFIRMED or DISTRIBUTING; got %v", got)
	}
}

func TestDecideVerdict_ExitCheckGatesConfirmed(t *testing.T) {
	// Same buyers, same full coverage: only whether the exchange side was
	// observed decides between CONFIRMED and INDEPENDENT.
	tests := []struct {
		name    string
		hasFlow bool
		want    Verdict
	}{
		{"exchange side observed, no inflow", true, VerdictConfirmed},
		{"no exchange address touched the token", false, VerdictIndependent},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DecideVerdict(4, 4, 4, 500, 0, tt.hasFlow); got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}

func TestDecideVerdict_MissingExchangeDataLeavesCollapseStanding(t *testing.T) {
	// A shared funder is a positive observation; the absent exit check
	// weakens only the negative claim CONFIRMED makes.
	if got := DecideVerdict(4, 2, 4, 500, 0, false); got != VerdictConcentrated {
		t.Fatalf("got %v want CONCENTRATED", got)
	}
	if got := DecideVerdict(4, 1, 4, 500, 0, false); got != VerdictThin {
		t.Fatalf("got %v want THIN", got)
	}
}

// TestDecideVerdict_Unverified is the bug the team lead caught: over the
// full seed (no --max-wallets cap), funder data is only cached for the
// enriched subset, so unchecked buyers fall into their own singleton
// cluster and M inflates toward N — an (N, M) shape that satisfies
// CONFIRMED's "no two buyers share a funder" test without that claim
// ever having been checked. Partial coverage at M==N must read
// UNVERIFIED, never CONFIRMED.
func TestDecideVerdict_Unverified(t *testing.T) {
	// N=4, M=4, but only 3 of the 4 buyers were actually checked.
	got := DecideVerdict(4, 4, 3, -100, -100, true)
	if got != VerdictUnverified {
		t.Fatalf("got %v want UNVERIFIED", got)
	}
}

func TestDecideVerdict_UnverifiedOverridesDistribution(t *testing.T) {
	// The exit signal needs no funder data (docs/DESIGN.md), so it still
	// fires on this input — but DISTRIBUTING asserts a confirmed
	// independence read on top of that, which incomplete coverage can't
	// support. UNVERIFIED wins over the distribution overlay; the raw
	// flow numbers next to the verdict still show the exit signal fired.
	got := DecideVerdict(4, 4, 3, 100, 100, true)
	if got != VerdictUnverified {
		t.Fatalf("got %v want UNVERIFIED (not DISTRIBUTING) with partial coverage", got)
	}
}

func TestDecideVerdict_ThinValidUnderPartialCoverage(t *testing.T) {
	// A positively observed shared funder (M==1) proves dependence
	// regardless of what else is missing — only the negative "no shared
	// funder" claim needs full coverage.
	got := DecideVerdict(5, 1, 2, -100, -100, true)
	if got != VerdictThin {
		t.Fatalf("got %v want THIN even at partial coverage", got)
	}
}

func TestDecideVerdict_ConcentratedValidUnderPartialCoverage(t *testing.T) {
	got := DecideVerdict(4, 2, 1, -100, -100, true)
	if got != VerdictConcentrated {
		t.Fatalf("got %v want CONCENTRATED even at partial coverage", got)
	}
}

// TestDecideVerdict_Total exercises every (N,M) shape the table must
// cover, per docs/DESIGN.md's "the rules below are total" claim,
// including the N>=3,M==2 case that fell through to WEAK in the first
// version (ETH 4->2, USDG 3->2, ZCAT 3->2 in the real cache). Coverage is
// full (covered == n) throughout except where the case name says
// otherwise, since coverage is exercised on its own above.
func TestDecideVerdict_Total(t *testing.T) {
	cases := []struct {
		name      string
		n, m, cov int
		smart, x  float64
		hasFlow   bool
		want      Verdict
	}{
		{"n=0", 0, 0, 0, 0, 0, false, VerdictWeak},
		{"n=1,m=1", 1, 1, 1, 0, 0, false, VerdictWeak},
		{"n=2,m=2", 2, 2, 2, 0, 0, false, VerdictWeak},
		{"n=1,m=1 exit fires", 1, 1, 1, 1, 1, true, VerdictExitOnly},
		{"n=2,m=2 exit fires", 2, 2, 2, 1, 1, true, VerdictExitOnly},
		{"eth 4->2 no dist", 4, 2, 4, -1, -1, true, VerdictConcentrated},
		{"usdg 3->2 no dist", 3, 2, 3, -1, -1, true, VerdictConcentrated},
		{"n=3,m=1 no dist", 3, 1, 3, -1, -1, true, VerdictThin},
		{"n=3,m=3 no dist, full coverage", 3, 3, 3, -1, -1, true, VerdictConfirmed},
		{"n=3,m=3 no dist, partial coverage", 3, 3, 2, -1, -1, true, VerdictUnverified},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DecideVerdict(c.n, c.m, c.cov, c.smart, c.x, c.hasFlow)
			if got != c.want {
				t.Fatalf("DecideVerdict(%d,%d,%d,...) = %v, want %v", c.n, c.m, c.cov, got, c.want)
			}
		})
	}
}
