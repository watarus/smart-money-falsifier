package score

// Verdict is the categorical falsifier label for one token, per
// docs/DESIGN.md's Verdict section. Verdicts replace the old continuous
// token score: the previous normalisation saturated (every rendered token
// scored 1.000), which is how that bug hid in plain sight.
type Verdict string

const (
	VerdictBoth         Verdict = "BOTH"
	VerdictThin         Verdict = "THIN"
	VerdictConcentrated Verdict = "CONCENTRATED"
	VerdictDistributing Verdict = "DISTRIBUTING"
	VerdictConfirmed    Verdict = "CONFIRMED"
	VerdictWeak         Verdict = "WEAK"

	// VerdictExitOnly is a token with fewer than 3 buyers (no independence
	// signal) whose exit signal fires anyway. docs/DESIGN.md: the buyer
	// floor gates the independence verdict only, never the row — one
	// smart-money wallet accumulating into exchange outflows is exactly as
	// damning as five. Callers render this token in the main table with
	// its independence column reading "—" rather than a verdict.
	VerdictExitOnly Verdict = "EXIT_ONLY"
)

// Distributing reports whether the exit signal fires: smart traders net
// buying while exchange-labelled addresses are simultaneously net
// receiving. hasFlow must be false when flow-intelligence data wasn't
// available for the token (a missing signal must never manufacture a
// DISTRIBUTING verdict).
func Distributing(smartTraderNetFlowUSD, exchangeNetFlowUSD float64, hasFlow bool) bool {
	if !hasFlow {
		return false
	}
	return smartTraderNetFlowUSD > 0 && exchangeNetFlowUSD > 0
}

// DecideVerdict applies docs/DESIGN.md's Verdict rules. The N >= 3 buyer
// floor gates the independence verdict only, never the exit signal:
//
//	THIN         M == 1        one actor wearing N hats
//	CONCENTRATED 1 < M < N      some of the apparent independence is illusory
//	CONFIRMED    M == N         no two buyers share a funder
//
// Then the distribution overlay: if the exit signal fires, CONFIRMED
// becomes DISTRIBUTING, and THIN/CONCENTRATED become BOTH. The rules are
// total — every (N, M) lands somewhere; there is no WEAK fallthrough for
// N >= 3 the way the first version had (see the DESIGN.md changelog for
// the N>=3,M==2 gap this closed).
//
// Below the N >= 3 floor there is no independence signal at all, but the
// exit signal still needs no minimum buyer count (DESIGN.md: one
// smart-money wallet accumulating into exchange outflows is exactly as
// damning as five), so it is checked regardless: VerdictExitOnly if it
// fires, VerdictWeak (no signal of any kind) if it doesn't.
func DecideVerdict(n, m int, smartTraderNetFlowUSD, exchangeNetFlowUSD float64, hasFlow bool) Verdict {
	dist := Distributing(smartTraderNetFlowUSD, exchangeNetFlowUSD, hasFlow)

	if n < 3 {
		if dist {
			return VerdictExitOnly
		}
		return VerdictWeak
	}

	switch {
	case m == 1:
		if dist {
			return VerdictBoth
		}
		return VerdictThin
	case m < n:
		if dist {
			return VerdictBoth
		}
		return VerdictConcentrated
	default: // m == n
		if dist {
			return VerdictDistributing
		}
		return VerdictConfirmed
	}
}
