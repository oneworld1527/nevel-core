package blockchain

import (
	"github.com/nevel/nevel-core/params"
	txpkg "github.com/nevel/nevel-core/tx"
)

const GenesisMessage = "NEVEL: The global money network begins."

func CreateGenesisBlock(net params.Network) Block {
	coinbase := txpkg.NewCoinbase(GenesisMessage, params.InitialReward, []byte("genesis"))
	block := Block{Header: BlockHeader{Version: 1, PrevHash: [32]byte{}, Timestamp: 1780000000, Bits: net.GenesisBits, Nonce: 0, Height: 0}, Transactions: []Transaction{coinbase}}
	block.Header.MerkleRoot = BuildMerkleRoot(block.Transactions)
	return block
}
