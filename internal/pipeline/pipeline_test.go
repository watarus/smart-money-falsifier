package pipeline

import (
	"testing"

	"github.com/watarus/nansen/internal/score"
)

// The signal floor withholds only the negative claim ("no shared funder").
// A shared funder that was actually observed stays reported however little
// was bought.
func TestDecideWithFloor(t *testing.T) {
	tests := []struct {
		name      string
		n, m      int
		smart     float64
		exch      float64
		exitCheck bool
		boughtUSD float64
		want      score.Verdict
	}{
		// RAFFLE in the real seed: four independent buyers, $156 between them.
		{"clean verdict on pocket change is withheld", 4, 4, 300, -50, true, 156, score.VerdictWeak},
		{"no-exchange verdict on pocket change is withheld", 4, 4, 300, 0, false, 156, score.VerdictWeak},
		{"shared funder below the floor still reported", 4, 2, 300, -50, true, 156, score.VerdictConcentrated},
		{"single funder below the floor still reported", 4, 1, 300, -50, true, 156, score.VerdictThin},
		{"shared funder plus exit below the floor still reported", 4, 2, 300, 200, true, 156, score.VerdictBoth},
		{"exit signal survives the floor when independence is withheld", 4, 4, 300, 200, true, 156, score.VerdictExitOnly},
		{"just under the floor", 4, 4, 300, -50, true, 999.99, score.VerdictWeak},
		{"at the floor", 4, 4, 300, -50, true, 1000, score.VerdictConfirmed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideWithFloor(tt.n, tt.m, tt.n, tt.smart, tt.exch, tt.exitCheck, tt.boughtUSD, score.MinSignalUSD)
			if got != tt.want {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
