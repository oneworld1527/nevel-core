package chain

import (
	"context"
	"testing"

	"github.com/nevel/nevel-core/mempool"
	"github.com/nevel/nevel-core/params"
	"github.com/nevel/nevel-core/storage"
	"github.com/nevel/nevel-core/wallet"
)

func TestManagerMinesAndIndexesBalance(t *testing.T) {
	db, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	net := params.Regtest()
	verifier := wallet.Verifier{}
	manager := New(db, net, mempool.New(params.DefaultMempoolMaxByte, db, verifier), verifier)
	if _, err := manager.InitGenesis(context.Background()); err != nil {
		t.Fatal(err)
	}
	w, err := wallet.New(net.AddressPrefix)
	if err != nil {
		t.Fatal(err)
	}
	mined, err := manager.MineBlocks(context.Background(), 1, w.Address)
	if err != nil {
		t.Fatal(err)
	}
	if len(mined) != 1 || mined[0].Height != 1 {
		t.Fatalf("unexpected mined tips: %#v", mined)
	}
	script, err := wallet.LockingScriptForAddress(w.Address, net.AddressPrefix)
	if err != nil {
		t.Fatal(err)
	}
	balance, err := db.BalanceByScript(script)
	if err != nil {
		t.Fatal(err)
	}
	if balance != params.InitialReward {
		t.Fatalf("balance=%d want %d", balance, params.InitialReward)
	}
}
