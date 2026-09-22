package pipeline

import (
	"testing"

	"github.com/watarus/nansen/internal/score"
)

func TestJudgeableBuyers(t *testing.T) {
	tests := []struct {
		name      string
		n         int
		boughtUSD float64
		want      int
	}{
		// RAFFLE in the real seed: four buyers, $156 between them.
		{"four buyers of pocket change are not a signal", 4, 156, 0},
		{"just under the floor", 4, 999.99, 0},
		{"at the floor", 4, 1000, 4},
		{"well above the floor", 5, 4750, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := judgeableBuyers(tt.n, tt.boughtUSD, score.MinSignalUSD); got != tt.want {
				t.Fatalf("judgeableBuyers(%d, %v) = %d, want %d", tt.n, tt.boughtUSD, got, tt.want)
			}
		})
	}
}

// Below the floor the token must reach DecideVerdict as unjudgeable, so a
// clean-looking 4 -> 4 on $156 can never come out CONFIRMED.
func TestSignalFloorWithholdsConfirmed(t *testing.T) {
	n := judgeableBuyers(4, 156, score.MinSignalUSD)
	if got := score.DecideVerdict(n, 4, 4, 300, -50, true); got == score.VerdictConfirmed {
		t.Fatalf("a $156 signal came out CONFIRMED")
	}
}
