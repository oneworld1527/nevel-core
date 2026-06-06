package wallet

import (
	"testing"

	"github.com/nevel/nevel-core/consensus"
	txpkg "github.com/nevel/nevel-core/tx"
)

func TestWalletSignsAndVerifies(t *testing.T) {
	w, err := New("rnevel")
	if err != nil {
		t.Fatal(err)
	}
	script, err := LockingScriptForAddress(w.Address, "rnevel")
	if err != nil {
		t.Fatal(err)
	}
	prev := [32]byte{1, 2, 3}
	tr := txpkg.Transaction{Version: 1, Inputs: []txpkg.Input{{PrevTxHash: prev, OutputIdx: 0}}, Outputs: []txpkg.Output{{Amount: 10, LockingScript: consensus.ScriptForPubKeyHash([]byte{9})}}}
	if err := w.SignInput(&tr, 0, script); err != nil {
		t.Fatal(err)
	}
	if !(Verifier{}).VerifyTransactionInput(tr, 0, script) {
		t.Fatal("signature did not verify")
	}
}
