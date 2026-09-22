// cluster.go implements the "cluster collapse" falsifier from
// docs/DESIGN.md: union wallets that bought the same token when they share
// a First-Funder address, or when one wallet's First Funder is itself
// another buyer of that token. Only the "First Funder" relation is a
// funding edge; every other relation string (Deployed Program, Deployed
// Contract, ...) is a contract artefact and is ignored, including any
// relation string not yet observed.
package score

import "sort"

// Funder is one First-Funder edge for a wallet: the funding address and
// whatever label Nansen attaches to it (often empty).
type Funder struct {
	Address string
	Label   string
}

// BuyerInput is one wallet that bought a given token, as seen by the
// clustering pass. Funders must already be filtered to relation ==
// "First Funder" by the caller (internal/pipeline) — cluster.go trusts
// its input rather than re-filtering a raw relation string, so a caller
// bug can't silently reintroduce a Deployed-* edge here.
type BuyerInput struct {
	Address string
	Label   string
	PnLUSD  float64
	WinRate float64
	Funders []Funder

	// FunderChecked is true when a related-wallets call for this wallet
	// actually succeeded, regardless of whether it returned a First-Funder
	// edge. It is false when the call never ran or failed (cache miss,
	// API error, or the wallet fell outside --max-wallets). Distinguishing
	// "checked, found nothing" from "never checked" is what lets Cluster
	// report real coverage instead of letting missing data silently
	// inflate M toward N — see ClusterResult.Covered.
	FunderChecked bool
}

// ClusterGroup is one collapsed group of buyers who trace to the same
// funding actor (or, for a singleton, to none).
type ClusterGroup struct {
	Members []BuyerInput // sorted by address

	// SharedFunder is set when two or more members share a literal funder
	// address; it names that funder so the report can point at it.
	SharedFunder *Funder

	// ViaBuyer is set when the collapse happened because one member's
	// funder is itself another buyer's own address (the funder never
	// appears as an independent third party), rather than because two
	// members share an external funder. Holds that buyer's address.
	ViaBuyer string

	// NoFunderData is true for a singleton cluster whose one member was
	// never actually checked for funder data (BuyerInput.FunderChecked ==
	// false) — a cache miss, a failed call, or a wallet outside
	// --max-wallets. A singleton whose funder data WAS fetched and simply
	// returned no First-Funder edge is a verified-independent wallet, not
	// a coverage gap, so it does not set this flag. Distinguishing this
	// keeps the report honest that missing data never manufactured (or
	// hid) a collapse.
	NoFunderData bool
}

// ClusterResult is the outcome of collapsing one token's buyer set.
type ClusterResult struct {
	N        int // distinct buying wallets
	M        int // distinct clusters after union
	Covered  int // buyers whose funder data was actually fetched (BuyerInput.FunderChecked)
	Clusters []ClusterGroup
}

// Cluster unions buyers of one token per docs/DESIGN.md's cluster-collapse
// rule and returns the collapsed groups, most populous first.
func Cluster(buyers []BuyerInput) ClusterResult {
	n := len(buyers)
	if n == 0 {
		return ClusterResult{}
	}

	idx := make(map[string]int, n)
	covered := 0
	for i, b := range buyers {
		idx[b.Address] = i
		if b.FunderChecked {
			covered++
		}
	}

	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}

	// funderOwners: funder address -> indices of buyers claiming it as
	// their First Funder. Two buyers sharing a funder collapse.
	funderOwners := map[string][]int{}
	for i, b := range buyers {
		for _, f := range b.Funders {
			funderOwners[f.Address] = append(funderOwners[f.Address], i)
		}
	}
	for _, owners := range funderOwners {
		for k := 1; k < len(owners); k++ {
			union(owners[0], owners[k])
		}
	}

	// A buyer's funder that is itself another buyer's address collapses
	// the two (one actor wearing two hats: funder, then funded).
	for i, b := range buyers {
		for _, f := range b.Funders {
			if j, ok := idx[f.Address]; ok && j != i {
				union(i, j)
			}
		}
	}

	groups := map[int][]int{}
	for i := range buyers {
		r := find(i)
		groups[r] = append(groups[r], i)
	}

	clusters := make([]ClusterGroup, 0, len(groups))
	for _, members := range groups {
		sort.Slice(members, func(a, b int) bool { return buyers[members[a]].Address < buyers[members[b]].Address })

		c := ClusterGroup{}
		for _, mi := range members {
			c.Members = append(c.Members, buyers[mi])
		}

		if len(members) == 1 {
			if !buyers[members[0]].FunderChecked {
				c.NoFunderData = true
			}
			clusters = append(clusters, c)
			continue
		}

		// Explain the collapse: prefer naming a literal shared funder
		// address (present in >=2 members' Funders); otherwise it must
		// have been a funder-is-a-buyer edge.
		memberSet := map[int]bool{}
		for _, mi := range members {
			memberSet[mi] = true
		}
		var shared *Funder
		for fa, owners := range funderOwners {
			count := 0
			for _, o := range owners {
				if memberSet[o] {
					count++
				}
			}
			if count >= 2 {
				f := Funder{Address: fa}
				for _, o := range owners {
					if memberSet[o] {
						for _, ff := range buyers[o].Funders {
							if ff.Address == fa && ff.Label != "" {
								f.Label = ff.Label
							}
						}
					}
				}
				shared = &f
				break
			}
		}
		if shared != nil {
			c.SharedFunder = shared
		} else {
			for _, mi := range members {
				for _, f := range buyers[mi].Funders {
					if j, ok := idx[f.Address]; ok && memberSet[j] {
						c.ViaBuyer = buyers[j].Address
						break
					}
				}
				if c.ViaBuyer != "" {
					break
				}
			}
		}
		clusters = append(clusters, c)
	}

	sort.Slice(clusters, func(i, j int) bool {
		if len(clusters[i].Members) != len(clusters[j].Members) {
			return len(clusters[i].Members) > len(clusters[j].Members)
		}
		return clusters[i].Members[0].Address < clusters[j].Members[0].Address
	})

	return ClusterResult{N: n, M: len(clusters), Covered: covered, Clusters: clusters}
}

// FunderCensus is the whole-universe view (not per-token) used to
// reproduce docs/DESIGN.md's headline numbers: how many funders are
// shared across more than one wallet, and who funds the most wallets.
type FunderCensus struct {
	DistinctFunders int
	SharedFunders   int // funders claimed by more than one wallet
	TopFunders      []FunderCount
}

// FunderCount is one funder address and how many wallets in the universe
// name it as their First Funder.
type FunderCount struct {
	Address     string
	Label       string
	WalletCount int
}

// ComputeFunderCensus tallies First-Funder addresses across every wallet in
// the universe (not scoped to a single token), sorted by wallet count
// descending then address, for the report's summary line and the sanity
// check against docs/DESIGN.md's "16 of 50 trace to 2 funders".
func ComputeFunderCensus(wallets []BuyerInput) FunderCensus {
	counts := map[string]*FunderCount{}
	for _, w := range wallets {
		seen := map[string]bool{}
		for _, f := range w.Funders {
			if seen[f.Address] {
				continue
			}
			seen[f.Address] = true
			fc, ok := counts[f.Address]
			if !ok {
				fc = &FunderCount{Address: f.Address, Label: f.Label}
				counts[f.Address] = fc
			}
			if fc.Label == "" && f.Label != "" {
				fc.Label = f.Label
			}
			fc.WalletCount++
		}
	}

	top := make([]FunderCount, 0, len(counts))
	shared := 0
	for _, fc := range counts {
		if fc.WalletCount > 1 {
			shared++
		}
		top = append(top, *fc)
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].WalletCount != top[j].WalletCount {
			return top[i].WalletCount > top[j].WalletCount
		}
		return top[i].Address < top[j].Address
	})

	return FunderCensus{DistinctFunders: len(counts), SharedFunders: shared, TopFunders: top}
}
