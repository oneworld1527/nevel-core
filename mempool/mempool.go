package mempool

import (
	"sort"
	"sync"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/consensus"
	txpkg "github.com/nevel/nevel-core/tx"
)

type Entry struct {
	Tx    txpkg.Transaction
	Fee   uint64
	Size  int64
	Added time.Time
}
type Mempool struct {
	mu       sync.RWMutex
	txs      map[[32]byte]Entry
	spends   map[string][32]byte
	maxBytes int64
	bytes    int64
	utxos    consensus.UTXOReader
	verifier consensus.SignatureVerifier
}

func New(maxBytes int64, utxos consensus.UTXOReader, verifier consensus.SignatureVerifier) *Mempool {
	return &Mempool{txs: map[[32]byte]Entry{}, spends: map[string][32]byte{}, maxBytes: maxBytes, utxos: utxos, verifier: verifier}
}
func (m *Mempool) Add(t txpkg.Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	spent := map[string]struct{}{}
	for k := range m.spends {
		spent[k] = struct{}{}
	}
	fee, err := consensus.ValidateTransaction(t, m.utxos, m.verifier, spent)
	if err != nil {
		return err
	}
	h := t.Hash()
	size := int64(len(txpkg.SerializeTransaction(t)))
	for m.bytes+size > m.maxBytes {
		if !m.evictLowest() {
			break
		}
	}
	m.txs[h] = Entry{Tx: t, Fee: fee, Size: size, Added: time.Now()}
	m.bytes += size
	for _, in := range t.Inputs {
		m.spends[consensus.OutpointKey(in.PrevTxHash, in.OutputIdx)] = h
	}
	return nil
}
func (m *Mempool) EntriesByFeeRate() []Entry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Entry, 0, len(m.txs))
	for _, e := range m.txs {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Fee*uint64(out[j].Size) > out[j].Fee*uint64(out[i].Size) })
	return out
}
func (m *Mempool) Snapshot() []txpkg.Transaction {
	entries := m.EntriesByFeeRate()
	out := make([]txpkg.Transaction, len(entries))
	for i, e := range entries {
		out[i] = e.Tx
	}
	return out
}

// FIX #4: RemoveMinedBlock removes all transactions that were included in a
// mined block from the mempool. This must be called after every block
// acceptance, including after reorgs.
func (m *Mempool) RemoveMinedBlock(b blockchain.Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range b.Transactions {
		if t.IsCoinbase() {
			continue
		}
		h := t.Hash()
		if e, ok := m.txs[h]; ok {
			delete(m.txs, h)
			m.bytes -= e.Size
			for _, in := range e.Tx.Inputs {
				delete(m.spends, consensus.OutpointKey(in.PrevTxHash, in.OutputIdx))
			}
		}
	}
}

// Remove removes a single transaction by hash.
func (m *Mempool) Remove(hash [32]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.txs[hash]
	if !ok {
		return
	}
	delete(m.txs, hash)
	m.bytes -= e.Size
	for _, in := range e.Tx.Inputs {
		delete(m.spends, consensus.OutpointKey(in.PrevTxHash, in.OutputIdx))
	}
}

func (m *Mempool) evictLowest() bool {
	var low [32]byte
	first := true
	for h, e := range m.txs {
		if first || e.Fee*uint64(m.txs[low].Size) < m.txs[low].Fee*uint64(e.Size) {
			low = h
			first = false
		}
	}
	if first {
		return false
	}
	e := m.txs[low]
	delete(m.txs, low)
	m.bytes -= e.Size
	for _, in := range e.Tx.Inputs {
		delete(m.spends, consensus.OutpointKey(in.PrevTxHash, in.OutputIdx))
	}
	return true
}

// Size returns the number of transactions currently in the mempool.
func (m *Mempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.txs)
}
