package chain

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/consensus"
	"github.com/nevel/nevel-core/mempool"
	"github.com/nevel/nevel-core/miner"
	"github.com/nevel/nevel-core/params"
	"github.com/nevel/nevel-core/storage"
	txpkg "github.com/nevel/nevel-core/tx"
	"github.com/nevel/nevel-core/wallet"
)

var ErrChainUninitialized = errors.New("chain is not initialized")

type Manager struct {
	Store    *storage.DB
	Network  params.Network
	Mempool  *mempool.Mempool
	Verifier consensus.SignatureVerifier
}

type Tip struct {
	Hash      string `json:"hash"`
	Height    uint64 `json:"height"`
	Bits      uint32 `json:"bits"`
	Timestamp int64  `json:"timestamp"`
}

type MiningTemplate struct {
	PreviousHash string   `json:"previousHash"`
	Height       uint64   `json:"height"`
	Bits         uint32   `json:"bits"`
	Target       string   `json:"target"`
	Transactions []string `json:"transactions"`
	Coinbase     uint64   `json:"coinbase"`
}

func New(store *storage.DB, net params.Network, mp *mempool.Mempool, verifier consensus.SignatureVerifier) *Manager {
	return &Manager{Store: store, Network: net, Mempool: mp, Verifier: verifier}
}

func (m *Manager) InitGenesis(ctx context.Context) (blockchain.Block, error) {
	if h, err := m.Store.Tip(); err == nil {
		return m.Store.GetBlock(h)
	}
	g := blockchain.CreateGenesisBlock(m.Network)
	if ok := miner.Mine(ctx, &g, m.Network.PowLimit); !ok {
		return blockchain.Block{}, ctx.Err()
	}
	if err := m.Store.ApplyBlock(g); err != nil {
		return blockchain.Block{}, err
	}
	gh := g.Hash()
	_ = m.Store.PutTotalWork(gh, consensus.WorkForBits(g.Header.Bits))
	return g, nil
}

func (m *Manager) Tip() (Tip, error) {
	hash, err := m.Store.Tip()
	if err != nil {
		return Tip{}, ErrChainUninitialized
	}
	b, err := m.Store.GetBlock(hash)
	if err != nil {
		return Tip{}, err
	}
	return Tip{Hash: hex.EncodeToString(hash[:]), Height: b.Header.Height, Bits: b.Header.Bits, Timestamp: b.Header.Timestamp}, nil
}

func (m *Manager) TipBlock() (blockchain.Block, error) {
	hash, err := m.Store.Tip()
	if err != nil {
		return blockchain.Block{}, ErrChainUninitialized
	}
	return m.Store.GetBlock(hash)
}

func (m *Manager) HeaderByHeight(height uint64) (blockchain.BlockHeader, error) {
	hash, err := m.Store.BlockHashByHeight(height)
	if err != nil {
		return blockchain.BlockHeader{}, err
	}
	b, err := m.Store.GetBlock(hash)
	if err != nil {
		return blockchain.BlockHeader{}, err
	}
	return b.Header, nil
}

func (m *Manager) NextBits(prev blockchain.BlockHeader) (uint32, error) {
	return consensus.ExpectedBits(prev, m.Network, m.HeaderByHeight)
}

func (m *Manager) BuildTemplate(minerScript []byte) (blockchain.Block, MiningTemplate, error) {
	prev, err := m.TipBlock()
	if err != nil {
		return blockchain.Block{}, MiningTemplate{}, err
	}
	prevHash := prev.Hash()
	height := prev.Header.Height + 1
	bits, err := m.NextBits(prev.Header)
	if err != nil {
		return blockchain.Block{}, MiningTemplate{}, err
	}
	txs := []txpkg.Transaction{}
	if m.Mempool != nil {
		txs = append(txs, m.Mempool.Snapshot()...)
	}
	coinbaseValue := consensus.BlockReward(height)
	coinbase := txpkg.NewCoinbase(fmt.Sprintf("NEVEL mined at %d", time.Now().Unix()), coinbaseValue, minerScript)
	blockTxs := append([]txpkg.Transaction{coinbase}, txs...)
	block := blockchain.NewBlock(prevHash, height, bits, blockTxs)
	hashes := make([]string, len(txs))
	for i, tx := range txs {
		h := tx.Hash()
		hashes[i] = hex.EncodeToString(h[:])
	}
	target := params.CompactToBig(bits)
	tmpl := MiningTemplate{PreviousHash: hex.EncodeToString(prevHash[:]), Height: height, Bits: bits, Target: target.Text(16), Transactions: hashes, Coinbase: coinbaseValue}
	return block, tmpl, nil
}

type MiningSummary struct {
	Blocks uint64
	First  Tip
	Last   Tip
}

func (m *Manager) MineBlocks(ctx context.Context, count int, address string) ([]Tip, error) {
	if count < 1 {
		return nil, errors.New("blocks must be greater than zero")
	}
	mined := make([]Tip, 0, count)
	_, err := m.MineBlocksStreaming(ctx, count, address, func(tip Tip) {
		mined = append(mined, tip)
	})
	return mined, err
}

func (m *Manager) MineBlocksStreaming(ctx context.Context, count int, address string, onBlock func(Tip)) (MiningSummary, error) {
	if count < 1 {
		return MiningSummary{}, errors.New("blocks must be greater than zero")
	}
	script, err := wallet.LockingScriptForAddress(address, m.Network.AddressPrefix)
	if err != nil {
		return MiningSummary{}, err
	}
	summary := MiningSummary{}
	for i := 0; i < count; i++ {
		block, _, err := m.BuildTemplate(script)
		if err != nil {
			return summary, err
		}
		if ok := miner.Mine(ctx, &block, params.CompactToBig(block.Header.Bits)); !ok {
			return summary, ctx.Err()
		}
		if err := m.ValidateAndApplyBlock(block); err != nil {
			return summary, err
		}
		h := block.Hash()
		tip := Tip{Hash: hex.EncodeToString(h[:]), Height: block.Header.Height, Bits: block.Header.Bits, Timestamp: block.Header.Timestamp}
		if summary.Blocks == 0 {
			summary.First = tip
		}
		summary.Blocks++
		summary.Last = tip
		if onBlock != nil {
			onBlock(tip)
		}
	}
	return summary, nil
}

func TotalBlockRewards(firstHeight, lastHeight uint64) (uint64, error) {
	if lastHeight < firstHeight {
		return 0, nil
	}
	var total uint64
	for height := firstHeight; height <= lastHeight; height++ {
		reward := consensus.BlockReward(height)
		if ^uint64(0)-total < reward {
			return 0, txpkg.ErrInflation
		}
		total += reward
	}
	return total, nil
}

// ValidateAndApplyBlock FIX #1 & #2: Full fork/reorg with total-work fork choice.
// If the incoming block builds on a non-tip parent, we compare cumulative work
// and reorg to the heavier chain when necessary.
func (m *Manager) ValidateAndApplyBlock(block blockchain.Block) error {
	blockHash := block.Hash()

	// Check if this block already exists.
	if _, err := m.Store.GetBlock(blockHash); err == nil {
		return errors.New("block already known")
	}

	// Locate the previous block.
	var prevHeader *blockchain.BlockHeader
	if block.Header.Height > 0 {
		prev, err := m.Store.GetBlock(block.Header.PrevHash)
		if err != nil {
			return fmt.Errorf("previous block not found: %w", err)
		}
		prevHeader = &prev.Header
	}

	// Full header + difficulty + transaction validation.
	if err := consensus.ValidateBlockHeaderWithDifficulty(block, prevHeader, m.Network, m.HeaderByHeight); err != nil {
		return err
	}
	if err := consensus.ValidateBlockTransactions(block, m.Store, m.Verifier); err != nil {
		return err
	}

	// Compute total work for the incoming chain tip.
	incomingWork := consensus.WorkForBits(block.Header.Bits)
	if block.Header.Height > 0 {
		prevWork, err := m.Store.TotalWork(block.Header.PrevHash)
		if err == nil {
			incomingWork = new(big.Int).Add(incomingWork, prevWork)
		}
	}

	// Get current tip work.
	currentTipHash, err := m.Store.Tip()
	if err != nil {
		// No tip yet — just apply directly.
		if err2 := m.Store.ApplyBlock(block); err2 != nil {
			return err2
		}
		return m.Store.PutTotalWork(blockHash, incomingWork)
	}

	currentWork, err := m.Store.TotalWork(currentTipHash)
	if err != nil {
		currentWork = big.NewInt(0)
	}

	// If the block extends the current tip, apply directly (happy path).
	if block.Header.PrevHash == currentTipHash {
		if err := m.Store.ApplyBlock(block); err != nil {
			return err
		}
		if err := m.Store.PutTotalWork(blockHash, incomingWork); err != nil {
			return err
		}
		// FIX #4: Remove mined transactions from the mempool.
		if m.Mempool != nil {
			m.Mempool.RemoveMinedBlock(block)
		}
		return nil
	}

	// The block is on a side branch. Only reorg if it has more total work.
	if incomingWork.Cmp(currentWork) <= 0 {
		// Side chain is not heavier — store the block but don't switch.
		// We still write total work so future comparisons work.
		if err := m.Store.PutTotalWork(blockHash, incomingWork); err != nil {
			return err
		}
		return nil
	}

	// FIX #1: The side chain has more work — perform a reorg.
	return m.reorg(block, blockHash, incomingWork)
}

// reorg rolls the main chain back to the fork point, then applies the heavier
// side-chain forward.
func (m *Manager) reorg(newTip blockchain.Block, newTipHash [32]byte, newTipWork *big.Int) error {
	// Collect the side chain from newTip back to its fork point.
	sideChain := []blockchain.Block{newTip}
	cur := newTip
	for {
		if cur.Header.Height == 0 {
			break
		}
		// Check whether the parent is already on the main chain.
		mainHash, err := m.Store.BlockHashByHeight(cur.Header.Height - 1)
		if err != nil {
			return fmt.Errorf("reorg: missing height %d: %w", cur.Header.Height-1, err)
		}
		if mainHash == cur.Header.PrevHash {
			// Parent is on the main chain — this is the fork point.
			break
		}
		// Walk one step further back on the side chain.
		parentBlock, err := m.Store.GetBlock(cur.Header.PrevHash)
		if err != nil {
			return fmt.Errorf("reorg: missing side block %x: %w", cur.Header.PrevHash, err)
		}
		sideChain = append(sideChain, parentBlock)
		cur = parentBlock
	}
	forkHeight := cur.Header.Height - 1
	if cur.Header.Height == 0 {
		forkHeight = 0
	}

	// Roll back the main chain to the fork point.
	if err := m.RollbackToHeight(forkHeight); err != nil {
		return fmt.Errorf("reorg rollback: %w", err)
	}

	// Apply side chain blocks from oldest to newest (they are collected newest-first).
	for i := len(sideChain) - 1; i >= 0; i-- {
		b := sideChain[i]
		if err := m.Store.ApplyBlock(b); err != nil {
			return fmt.Errorf("reorg apply: %w", err)
		}
		bh := b.Hash()
		work := consensus.WorkForBits(b.Header.Bits)
		if b.Header.Height > 0 {
			pw, err := m.Store.TotalWork(b.Header.PrevHash)
			if err == nil {
				work = new(big.Int).Add(work, pw)
			}
		}
		if err := m.Store.PutTotalWork(bh, work); err != nil {
			return err
		}
	}

	// FIX #4: After reorg, remove mined transactions from mempool.
	if m.Mempool != nil {
		for _, b := range sideChain {
			m.Mempool.RemoveMinedBlock(b)
		}
	}

	return nil
}

func (m *Manager) RollbackToHeight(height uint64) error {
	for {
		tip, err := m.TipBlock()
		if err != nil {
			return err
		}
		if tip.Header.Height <= height {
			return nil
		}
		if _, err := m.Store.DisconnectTip(); err != nil {
			return err
		}
	}
}
