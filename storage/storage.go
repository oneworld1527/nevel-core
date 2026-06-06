package storage

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"path/filepath"

	"github.com/cockroachdb/pebble"
	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/consensus"
	txpkg "github.com/nevel/nevel-core/tx"
)

var ErrNotFound = errors.New("not found")

type UTXO = consensus.UTXO
type DB struct{ peb *pebble.DB }

func Open(path string) (*DB, error) {
	p := filepath.Clean(path)
	db, err := pebble.Open(p, &pebble.Options{})
	if err != nil {
		return nil, err
	}
	return &DB{peb: db}, nil
}
func (d *DB) Close() error { return d.peb.Close() }

func (d *DB) PutTotalWork(hash [32]byte, work *big.Int) error {
	return d.peb.Set(keyTotalWork(hash), []byte(work.String()), pebble.Sync)
}

func (d *DB) TotalWork(hash [32]byte) (*big.Int, error) {
	v, c, err := d.peb.Get(keyTotalWork(hash))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return big.NewInt(0), ErrNotFound
		}
		return nil, err
	}
	defer c.Close()
	work, ok := new(big.Int).SetString(string(v), 10)
	if !ok {
		return nil, errors.New("corrupt total work")
	}
	return work, nil
}

func (d *DB) SetTip(hash [32]byte) error {
	tip := make([]byte, 32)
	copy(tip, hash[:])
	return d.peb.Set([]byte("chain:tip"), tip, pebble.Sync)
}

func (d *DB) PutBlock(b blockchain.Block) error {
	hash := b.Hash()
	batch := d.peb.NewBatch()
	defer batch.Close()
	data := blockchain.SerializeBlock(b)
	if err := batch.Set(keyBlockHash(hash), data, pebble.Sync); err != nil {
		return err
	}
	hbuf := make([]byte, 32)
	copy(hbuf, hash[:])
	if err := batch.Set(keyBlockHeight(b.Header.Height), hbuf, pebble.Sync); err != nil {
		return err
	}
	for _, t := range b.Transactions {
		th := t.Hash()
		if err := batch.Set(keyTx(th), txpkg.SerializeTransaction(t), pebble.Sync); err != nil {
			return err
		}
	}
	tip := make([]byte, 32)
	copy(tip, hash[:])
	if err := batch.Set([]byte("chain:tip"), tip, pebble.Sync); err != nil {
		return err
	}
	return batch.Commit(pebble.Sync)
}
func (d *DB) GetBlock(hash [32]byte) (blockchain.Block, error) {
	v, closer, err := d.peb.Get(keyBlockHash(hash))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return blockchain.Block{}, ErrNotFound
		}
		return blockchain.Block{}, err
	}
	defer closer.Close()
	data := append([]byte(nil), v...)
	return blockchain.DeserializeBlock(data)
}
func (d *DB) BlockHashByHeight(height uint64) ([32]byte, error) {
	var h [32]byte
	v, c, err := d.peb.Get(keyBlockHeight(height))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return h, ErrNotFound
		}
		return h, err
	}
	defer c.Close()
	copy(h[:], v)
	return h, nil
}
func (d *DB) Tip() ([32]byte, error) {
	var h [32]byte
	v, c, err := d.peb.Get([]byte("chain:tip"))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return h, ErrNotFound
		}
		return h, err
	}
	defer c.Close()
	copy(h[:], v)
	return h, nil
}
func (d *DB) Get(hash [32]byte, idx uint32) (consensus.UTXO, error) {
	var u UTXO
	v, c, err := d.peb.Get(keyUTXO(hash, idx))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return u, ErrNotFound
		}
		return u, err
	}
	defer c.Close()
	data := append([]byte(nil), v...)
	return u, json.Unmarshal(data, &u)
}

func (d *DB) GetTx(hash [32]byte) (txpkg.Transaction, error) {
	v, c, err := d.peb.Get(keyTx(hash))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return txpkg.Transaction{}, ErrNotFound
		}
		return txpkg.Transaction{}, err
	}
	defer c.Close()
	data := append([]byte(nil), v...)
	return txpkg.DeserializeTransaction(data)
}

func (d *DB) Height() (uint64, error) {
	h, err := d.Tip()
	if err != nil {
		return 0, err
	}
	b, err := d.GetBlock(h)
	if err != nil {
		return 0, err
	}
	return b.Header.Height, nil
}

func (d *DB) ListUTXOs() ([]UTXO, error) {
	var out []UTXO
	err := d.peb.WalkPrefix([]byte("utxo:"), func(_ []byte, value []byte) error {
		var u UTXO
		if err := json.Unmarshal(value, &u); err != nil {
			return err
		}
		out = append(out, u)
		return nil
	})
	return out, err
}

func (d *DB) ListUTXOsByScript(script []byte) ([]UTXO, error) {
	all, err := d.ListUTXOs()
	if err != nil {
		return nil, err
	}
	out := make([]UTXO, 0)
	for _, u := range all {
		if string(u.Script) == string(script) {
			out = append(out, u)
		}
	}
	return out, nil
}

func (d *DB) BalanceByScript(script []byte) (uint64, error) {
	utxos, err := d.ListUTXOsByScript(script)
	if err != nil {
		return 0, err
	}
	var total uint64
	for _, u := range utxos {
		if ^uint64(0)-total < u.Amount {
			return 0, txpkg.ErrInflation
		}
		total += u.Amount
	}
	return total, nil
}

func (d *DB) PutUndo(hash [32]byte, undo []UTXO) error {
	data, err := json.Marshal(undo)
	if err != nil {
		return err
	}
	return d.peb.Set(keyUndo(hash), data, pebble.Sync)
}

func (d *DB) Undo(hash [32]byte) ([]UTXO, error) {
	v, c, err := d.peb.Get(keyUndo(hash))
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer c.Close()
	var undo []UTXO
	return undo, json.Unmarshal(v, &undo)
}

func (d *DB) DisconnectTip() (blockchain.Block, error) {
	hash, err := d.Tip()
	if err != nil {
		return blockchain.Block{}, err
	}
	b, err := d.GetBlock(hash)
	if err != nil {
		return blockchain.Block{}, err
	}
	undo, err := d.Undo(hash)
	if err != nil {
		return blockchain.Block{}, err
	}

	// FIX #3: Use a single atomic batch for the entire DisconnectTip operation.
	batch := d.peb.NewBatch()
	defer batch.Close()

	for _, t := range b.Transactions {
		th := t.Hash()
		for i := range t.Outputs {
			if err := batch.Delete(keyUTXO(th, uint32(i)), pebble.Sync); err != nil {
				return blockchain.Block{}, err
			}
		}
	}
	for _, u := range undo {
		data, err := json.Marshal(u)
		if err != nil {
			return blockchain.Block{}, err
		}
		if err := batch.Set(keyUTXO(u.TxHash, u.OutputIdx), data, pebble.Sync); err != nil {
			return blockchain.Block{}, err
		}
	}
	if b.Header.Height == 0 {
		if err := batch.Delete([]byte("chain:tip"), pebble.Sync); err != nil {
			return blockchain.Block{}, err
		}
	} else {
		prevTip := make([]byte, 32)
		copy(prevTip, b.Header.PrevHash[:])
		if err := batch.Set([]byte("chain:tip"), prevTip, pebble.Sync); err != nil {
			return blockchain.Block{}, err
		}
	}
	return b, batch.Commit(pebble.Sync)
}

func (d *DB) PutUTXO(u UTXO) error {
	data, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return d.peb.Set(keyUTXO(u.TxHash, u.OutputIdx), data, pebble.Sync)
}
func (d *DB) SpendUTXO(hash [32]byte, idx uint32) error {
	batch := d.peb.NewBatch()
	defer batch.Close()
	if err := batch.Delete(keyUTXO(hash, idx), pebble.Sync); err != nil {
		return err
	}
	if err := batch.Set(keySpent(hash, idx), []byte{1}, pebble.Sync); err != nil {
		return err
	}
	return batch.Commit(pebble.Sync)
}

// ApplyBlock FIX #3: Fully atomic — undo log, UTXO mutations, and tip pointer
// all commit in a single Pebble batch. Nothing is visible until the commit.
func (d *DB) ApplyBlock(b blockchain.Block) error {
	var undo []UTXO
	for _, t := range b.Transactions {
		if t.IsCoinbase() {
			continue
		}
		for _, in := range t.Inputs {
			u, err := d.Get(in.PrevTxHash, in.OutputIdx)
			if err == nil {
				undo = append(undo, u)
			}
		}
	}

	batch := d.peb.NewBatch()
	defer batch.Close()

	// Write undo log.
	undoData, err := json.Marshal(undo)
	if err != nil {
		return err
	}
	if err := batch.Set(keyUndo(b.Hash()), undoData, pebble.Sync); err != nil {
		return err
	}

	// Write block + height index + transactions.
	hash := b.Hash()
	blockData := blockchain.SerializeBlock(b)
	if err := batch.Set(keyBlockHash(hash), blockData, pebble.Sync); err != nil {
		return err
	}
	hbuf := make([]byte, 32)
	copy(hbuf, hash[:])
	if err := batch.Set(keyBlockHeight(b.Header.Height), hbuf, pebble.Sync); err != nil {
		return err
	}
	for _, t := range b.Transactions {
		th := t.Hash()
		if err := batch.Set(keyTx(th), txpkg.SerializeTransaction(t), pebble.Sync); err != nil {
			return err
		}
	}

	// Spend inputs and create outputs — all inside the same batch.
	for ti, t := range b.Transactions {
		th := t.Hash()
		if !t.IsCoinbase() {
			for _, in := range t.Inputs {
				if err := batch.Delete(keyUTXO(in.PrevTxHash, in.OutputIdx), pebble.Sync); err != nil {
					return err
				}
				if err := batch.Set(keySpent(in.PrevTxHash, in.OutputIdx), []byte{1}, pebble.Sync); err != nil {
					return err
				}
			}
		}
		for i, out := range t.Outputs {
			u := UTXO{TxHash: th, OutputIdx: uint32(i), Amount: out.Amount, Script: out.LockingScript, Height: b.Header.Height, Coinbase: ti == 0}
			data, err := json.Marshal(u)
			if err != nil {
				return err
			}
			if err := batch.Set(keyUTXO(th, uint32(i)), data, pebble.Sync); err != nil {
				return err
			}
		}
	}

	// Advance the tip pointer — last thing in the batch.
	tip := make([]byte, 32)
	copy(tip, hash[:])
	if err := batch.Set([]byte("chain:tip"), tip, pebble.Sync); err != nil {
		return err
	}

	return batch.Commit(pebble.Sync)
}
func keyBlockHeight(height uint64) []byte {
	b := make([]byte, 8)
	binary.BigEndian.PutUint64(b, height)
	return []byte("block:" + hex.EncodeToString(b))
}
func keyBlockHash(hash [32]byte) []byte         { return []byte("blockhash:" + hex.EncodeToString(hash[:])) }
func keyTx(hash [32]byte) []byte                { return []byte("tx:" + hex.EncodeToString(hash[:])) }
func keyUTXO(hash [32]byte, idx uint32) []byte  { return []byte(fmt.Sprintf("utxo:%x:%d", hash, idx)) }
func keySpent(hash [32]byte, idx uint32) []byte { return []byte(fmt.Sprintf("spent:%x:%d", hash, idx)) }
func keyTotalWork(hash [32]byte) []byte {
	return []byte("chain:totalwork:" + hex.EncodeToString(hash[:]))
}
func keyUndo(hash [32]byte) []byte { return []byte("undo:" + hex.EncodeToString(hash[:])) }
