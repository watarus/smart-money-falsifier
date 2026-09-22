package pipeline

import (
	"os"
	"path/filepath"

	"github.com/watarus/nansen/internal/nansen"
)

// SeedRequest is the one dex-trades request the whole pipeline is built
// around, per docs/DESIGN.md: {"chains": ["all"], "pagination": {"page": 1,
// "per_page": 1000}}.
var SeedRequest = nansen.DexTradesRequest{
	Chains:     []string{"all"},
	Pagination: nansen.PaginationReq{Page: 1, PerPage: 1000},
}

// ImportSeedFixture pre-populates the disk cache from
// data/fixtures/smart-money_dex-trades.json when the seed call isn't
// cached yet, so --dry-run and --offline (and a real first run) never
// need to spend the seed's 5 credits again — that response was already
// paid for once and is byte-for-byte the same request. It is a no-op
// (not an error) if fixturePath doesn't exist or the seed is already
// cached.
func ImportSeedFixture(cache *nansen.Cache, fixturePath string) error {
	key, err := cache.Key(nansen.PathSmartMoneyDexTrades, SeedRequest)
	if err != nil {
		return err
	}
	if cache.Has(key) {
		return nil
	}
	raw, err := os.ReadFile(fixturePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return cache.Put(key, raw)
}

// DefaultFixturePath resolves the seed fixture path relative to a repo
// root.
func DefaultFixturePath(repoRoot string) string {
	return filepath.Join(repoRoot, "data", "fixtures", "smart-money_dex-trades.json")
}
