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

	// VerdictUnverified is an (N, M) shape that would satisfy CONFIRMED's
	// "no two buyers share a funder" test, but not every buyer's funder
	// data was actually fetched — so that negative claim was never fully
	// checked. docs/DESIGN.md's Cluster collapse section: a wallet with no
	// funder data is its own cluster so absent data never manufactures a
	// collapse, but the same absent data must also never manufacture the
	// opposite claim, independence, by inflating M toward N. CONFIRMED
	// requires full coverage; partial coverage with no shared funder found
	// is UNVERIFIED instead.
	VerdictUnverified Verdict = "UNVERIFIED"

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
//	THIN         M == 1        every buyer shares one funding source
//	CONCENTRATED 1 < M < N      some of the apparent independence is illusory
//	CONFIRMED    M == N, full coverage      no two buyers share a funder
//	UNVERIFIED   M == N, partial coverage   that claim was never fully checked
//
// covered is how many of the n buyers actually had funder data fetched
// (score.Cluster's Covered). THIN and CONCENTRATED need no coverage check:
// a shared funder that was positively observed proves dependence
// regardless of what else is missing — union only happens on data that
// exists. CONFIRMED's claim is the negative one, "no two buyers share a
// funder", which absent data can silently satisfy by inflating M toward N
// without a single funder actually having been checked; that claim
// requires full coverage or it is UNVERIFIED instead. The distribution
// overlay only applies to a genuinely confirmed independence read, so it
// does not reach the UNVERIFIED case — the raw flow numbers are always
// rendered alongside the verdict regardless (docs/DESIGN.md's Exit
// signal section), so the signal isn't hidden, just not promoted to a
// verdict that would overstate what was checked.
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
func DecideVerdict(n, m, covered int, smartTraderNetFlowUSD, exchangeNetFlowUSD float64, hasFlow bool) Verdict {
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
		if covered < n {
			return VerdictUnverified
		}
		if dist {
			return VerdictDistributing
		}
		return VerdictConfirmed
	}
}
