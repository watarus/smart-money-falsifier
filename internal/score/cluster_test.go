package score

import "testing"

func clusterSizes(r ClusterResult) []int {
	out := make([]int, len(r.Clusters))
	for i, c := range r.Clusters {
		out[i] = len(c.Members)
	}
	return out
}

func TestCluster_SharedFunderCollapses(t *testing.T) {
	buyers := []BuyerInput{
		{Address: "a", Funders: []Funder{{Address: "F", Label: "funder"}}},
		{Address: "b", Funders: []Funder{{Address: "F", Label: "funder"}}},
	}
	r := Cluster(buyers)
	if r.N != 2 || r.M != 1 {
		t.Fatalf("expected 2 buyers -> 1 cluster, got N=%d M=%d", r.N, r.M)
	}
	if r.Clusters[0].SharedFunder == nil || r.Clusters[0].SharedFunder.Address != "F" {
		t.Fatalf("expected SharedFunder F, got %+v", r.Clusters[0].SharedFunder)
	}
}

func TestCluster_DistinctFundersDoNotCollapse(t *testing.T) {
	buyers := []BuyerInput{
		{Address: "a", Funders: []Funder{{Address: "F1"}}},
		{Address: "b", Funders: []Funder{{Address: "F2"}}},
	}
	r := Cluster(buyers)
	if r.N != 2 || r.M != 2 {
		t.Fatalf("expected 2 buyers -> 2 clusters, got N=%d M=%d", r.N, r.M)
	}
}

func TestCluster_NoFunderDataStaysOwnCluster(t *testing.T) {
	buyers := []BuyerInput{
		{Address: "a", Funders: nil},
		{Address: "b", Funders: []Funder{{Address: "F"}}},
	}
	r := Cluster(buyers)
	if r.N != 2 || r.M != 2 {
		t.Fatalf("expected 2 buyers -> 2 clusters, got N=%d M=%d", r.N, r.M)
	}
	var noData *ClusterGroup
	for i := range r.Clusters {
		if r.Clusters[i].Members[0].Address == "a" {
			noData = &r.Clusters[i]
		}
	}
	if noData == nil || !noData.NoFunderData {
		t.Fatalf("expected wallet with no funder data to be flagged NoFunderData, got %+v", r.Clusters)
	}
}

func TestCluster_NonFirstFunderRelationIgnoredByCaller(t *testing.T) {
	// Cluster trusts its input is already filtered to First-Funder edges;
	// this test documents that a Deployed-* relation must never reach
	// Cluster as a Funder in the first place (internal/pipeline's job),
	// not that Cluster re-filters by relation string.
	buyers := []BuyerInput{
		{Address: "a", Funders: nil}, // a Deployed Program edge was filtered out upstream
		{Address: "b", Funders: nil},
	}
	r := Cluster(buyers)
	if r.M != 2 {
		t.Fatalf("expected two wallets with no First-Funder edges to stay independent, got M=%d", r.M)
	}
}

func TestCluster_FunderIsAnotherBuyer(t *testing.T) {
	// b's First Funder is a, and a is itself a buyer of the same token:
	// one actor funded themselves into a second wallet.
	buyers := []BuyerInput{
		{Address: "a", Funders: nil},
		{Address: "b", Funders: []Funder{{Address: "a"}}},
	}
	r := Cluster(buyers)
	if r.N != 2 || r.M != 1 {
		t.Fatalf("expected 2 buyers -> 1 cluster (funder is a buyer), got N=%d M=%d", r.N, r.M)
	}
	if r.Clusters[0].ViaBuyer != "a" {
		t.Fatalf("expected ViaBuyer=a, got %q", r.Clusters[0].ViaBuyer)
	}
}

func TestCluster_TransitiveSharingCollapsesToOne(t *testing.T) {
	// A and B share funder f1; B and C share funder f2. Transitively, all
	// three collapse into a single cluster even though A and C share no
	// funder directly.
	buyers := []BuyerInput{
		{Address: "A", Funders: []Funder{{Address: "f1"}}},
		{Address: "B", Funders: []Funder{{Address: "f1"}, {Address: "f2"}}},
		{Address: "C", Funders: []Funder{{Address: "f2"}}},
	}
	r := Cluster(buyers)
	if r.N != 3 || r.M != 1 {
		t.Fatalf("expected 3 buyers -> 1 cluster via transitive sharing, got N=%d M=%d", r.N, r.M)
	}
	if len(r.Clusters[0].Members) != 3 {
		t.Fatalf("expected all 3 members in the single cluster, got %+v", r.Clusters[0].Members)
	}
}

func TestComputeFunderCensus(t *testing.T) {
	wallets := []BuyerInput{
		{Address: "a", Funders: []Funder{{Address: "F", Label: "whale"}}},
		{Address: "b", Funders: []Funder{{Address: "F", Label: "whale"}}},
		{Address: "c", Funders: []Funder{{Address: "G"}}},
		{Address: "d", Funders: nil},
	}
	census := ComputeFunderCensus(wallets)
	if census.DistinctFunders != 2 {
		t.Fatalf("expected 2 distinct funders, got %d", census.DistinctFunders)
	}
	if census.SharedFunders != 1 {
		t.Fatalf("expected 1 shared funder, got %d", census.SharedFunders)
	}
	if len(census.TopFunders) == 0 || census.TopFunders[0].Address != "F" || census.TopFunders[0].WalletCount != 2 {
		t.Fatalf("expected top funder F with count 2, got %+v", census.TopFunders)
	}
}
