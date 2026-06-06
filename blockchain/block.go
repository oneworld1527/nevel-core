package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	txpkg "github.com/nevel/nevel-core/tx"
)

type Transaction = txpkg.Transaction
type Block struct {
	Header       BlockHeader
	Transactions []Transaction
}
type BlockHeader struct {
	Version    uint32
	PrevHash   [32]byte
	MerkleRoot [32]byte
	Timestamp  int64
	Bits       uint32
	Nonce      uint64
	Height     uint64
}

func (h BlockHeader) Hash() [32]byte {
	b := make([]byte, 0, 88)
	temp := make([]byte, 8)
	binary.LittleEndian.PutUint32(temp[:4], h.Version)
	b = append(b, temp[:4]...)
	b = append(b, h.PrevHash[:]...)
	b = append(b, h.MerkleRoot[:]...)
	binary.LittleEndian.PutUint64(temp, uint64(h.Timestamp))
	b = append(b, temp...)
	binary.LittleEndian.PutUint32(temp[:4], h.Bits)
	b = append(b, temp[:4]...)
	binary.LittleEndian.PutUint64(temp, h.Nonce)
	b = append(b, temp...)
	binary.LittleEndian.PutUint64(temp, h.Height)
	b = append(b, temp...)
	first := sha256.Sum256(b)
	return sha256.Sum256(first[:])
}
func (b Block) Hash() [32]byte { return b.Header.Hash() }
func BuildMerkleRoot(txs []Transaction) [32]byte {
	if len(txs) == 0 {
		return [32]byte{}
	}
	hashes := make([][32]byte, len(txs))
	for i, t := range txs {
		hashes[i] = t.Hash()
	}
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}
		next := make([][32]byte, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			data := append(hashes[i][:], hashes[i+1][:]...)
			first := sha256.Sum256(data)
			next = append(next, sha256.Sum256(first[:]))
		}
		hashes = next
	}
	return hashes[0]
}
func NewBlock(prev [32]byte, height uint64, bits uint32, txs []Transaction) Block {
	blk := Block{Header: BlockHeader{Version: 1, PrevHash: prev, Timestamp: time.Now().Unix(), Bits: bits, Height: height}, Transactions: txs}
	blk.Header.MerkleRoot = BuildMerkleRoot(txs)
	return blk
}
func SerializeBlock(b Block) []byte {
	var buf bytes.Buffer
	_ = WriteBlock(&buf, b)
	return buf.Bytes()
}
func WriteBlock(w io.Writer, b Block) error {
	if err := WriteHeader(w, b.Header); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, uint64(len(b.Transactions))); err != nil {
		return err
	}
	for _, t := range b.Transactions {
		if err := txpkg.WriteTransaction(w, t); err != nil {
			return err
		}
	}
	return nil
}
func WriteHeader(w io.Writer, h BlockHeader) error {
	fields := []any{h.Version, h.PrevHash, h.MerkleRoot, h.Timestamp, h.Bits, h.Nonce, h.Height}
	for _, f := range fields {
		if err := binary.Write(w, binary.LittleEndian, f); err != nil {
			return err
		}
	}
	return nil
}
func DeserializeBlock(data []byte) (Block, error) { return ReadBlock(bytes.NewReader(data)) }
func ReadBlock(r io.Reader) (Block, error) {
	var b Block
	if err := binary.Read(r, binary.LittleEndian, &b.Header.Version); err != nil {
		return b, err
	}
	if _, err := io.ReadFull(r, b.Header.PrevHash[:]); err != nil {
		return b, err
	}
	if _, err := io.ReadFull(r, b.Header.MerkleRoot[:]); err != nil {
		return b, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Header.Timestamp); err != nil {
		return b, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Header.Bits); err != nil {
		return b, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Header.Nonce); err != nil {
		return b, err
	}
	if err := binary.Read(r, binary.LittleEndian, &b.Header.Height); err != nil {
		return b, err
	}
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return b, err
	}
	if n > 100000 {
		return b, fmt.Errorf("too many block transactions: %d", n)
	}
	b.Transactions = make([]Transaction, n)
	for i := range b.Transactions {
		t, err := txpkg.ReadTransaction(r)
		if err != nil {
			return b, err
		}
		b.Transactions[i] = t
	}
	return b, nil
}
