package miner

import (
	"context"
	"math/big"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/params"
)

func Mine(ctx context.Context, block *blockchain.Block, target *big.Int) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			hash := block.Header.Hash()
			if new(big.Int).SetBytes(hash[:]).Cmp(target) < 0 {
				return true
			}
			block.Header.Nonce++
		}
	}
}
func MineWithBits(ctx context.Context, block *blockchain.Block) bool {
	return Mine(ctx, block, params.CompactToBig(block.Header.Bits))
}
