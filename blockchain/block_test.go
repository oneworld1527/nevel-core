package blockchain

import (
	"testing"

	"github.com/nevel/nevel-core/params"
)

func TestGenesisMerkleRootStable(t *testing.T) {
	g := CreateGenesisBlock(params.Regtest())
	if g.Header.MerkleRoot != BuildMerkleRoot(g.Transactions) {
		t.Fatal("bad merkle root")
	}
	data := SerializeBlock(g)
	decoded, err := DeserializeBlock(data)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Hash() != g.Hash() {
		t.Fatal("block hash changed after round trip")
	}
}
