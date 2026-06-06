package consensus

import (
	"math/big"
	"testing"

	"github.com/nevel/nevel-core/params"
)

func TestBlockRewardHalving(t *testing.T) {
	if got := BlockReward(0); got != params.InitialReward {
		t.Fatalf("reward height 0=%d", got)
	}
	if got := BlockReward(params.HalvingInterval); got != params.InitialReward/2 {
		t.Fatalf("reward first halving=%d", got)
	}
}
func TestRetargetClamps(t *testing.T) {
	old := big.NewInt(1000)
	got := Retarget(old, 1, 100)
	if got.Cmp(big.NewInt(250)) != 0 {
		t.Fatalf("min clamp target=%s", got)
	}
	got = Retarget(old, 1000, 100)
	if got.Cmp(big.NewInt(4000)) != 0 {
		t.Fatalf("max clamp target=%s", got)
	}
}
