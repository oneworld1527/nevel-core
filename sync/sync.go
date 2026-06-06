package sync

import (
	"errors"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/chain"
)

type HeaderSource interface {
	Headers(locator [][32]byte, stop [32]byte) ([]blockchain.BlockHeader, error)
	Block(hash [32]byte) (blockchain.Block, error)
}

type Synchronizer struct{ Chain *chain.Manager }

type Result struct {
	Headers   int
	Blocks    int
	TipHeight uint64
}

func (s Synchronizer) HeaderFirstSync(source HeaderSource) (Result, error) {
	if s.Chain == nil {
		return Result{}, errors.New("chain manager required")
	}
	tip, err := s.Chain.TipBlock()
	if err != nil {
		return Result{}, err
	}
	locator := [][32]byte{tip.Hash()}
	headers, err := source.Headers(locator, [32]byte{})
	if err != nil {
		return Result{}, err
	}
	result := Result{Headers: len(headers), TipHeight: tip.Header.Height}
	expectedPrev := tip.Hash()
	for _, header := range headers {
		if header.PrevHash != expectedPrev {
			return result, errors.New("non-contiguous header chain")
		}
		block, err := source.Block(header.Hash())
		if err != nil {
			return result, err
		}
		if block.Header.Hash() != header.Hash() {
			return result, errors.New("block/header hash mismatch")
		}
		if err := s.Chain.ValidateAndApplyBlock(block); err != nil {
			return result, err
		}
		result.Blocks++
		result.TipHeight = block.Header.Height
		expectedPrev = header.Hash()
	}
	return result, nil
}
