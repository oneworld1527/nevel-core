package tx

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	ErrDustOutput = errors.New("dust output")
	ErrInflation  = errors.New("outputs exceed inputs")
	ErrFeeTooLow  = errors.New("fee too low")
)

type Transaction struct {
	Version  uint32
	Inputs   []Input
	Outputs  []Output
	LockTime uint64
}
type Input struct {
	PrevTxHash [32]byte
	OutputIdx  uint32
	Signature  []byte
	PublicKey  []byte
}
type Output struct {
	Amount        uint64
	LockingScript []byte
}

func (t Transaction) Hash() [32]byte {
	encoded := SerializeTransaction(t)
	first := sha256.Sum256(encoded)
	return sha256.Sum256(first[:])
}
func (t Transaction) IsCoinbase() bool {
	return len(t.Inputs) == 1 && t.Inputs[0].PrevTxHash == [32]byte{} && t.Inputs[0].OutputIdx == ^uint32(0)
}
func (t Transaction) Fee(inputTotal uint64) (uint64, error) {
	var out uint64
	for _, o := range t.Outputs {
		if ^uint64(0)-out < o.Amount {
			return 0, ErrInflation
		}
		out += o.Amount
	}
	if out > inputTotal {
		return 0, ErrInflation
	}
	return inputTotal - out, nil
}

func NewCoinbase(message string, amount uint64, script []byte) Transaction {
	return Transaction{Version: 1, Inputs: []Input{{OutputIdx: ^uint32(0), Signature: []byte(message)}}, Outputs: []Output{{Amount: amount, LockingScript: script}}}
}

func SerializeTransaction(t Transaction) []byte {
	var b bytes.Buffer
	_ = WriteTransaction(&b, t)
	return b.Bytes()
}
func WriteTransaction(w io.Writer, t Transaction) error {
	if err := binary.Write(w, binary.LittleEndian, t.Version); err != nil {
		return err
	}
	if err := writeVarBytesList(w, len(t.Inputs)); err != nil {
		return err
	}
	for _, in := range t.Inputs {
		if _, err := w.Write(in.PrevTxHash[:]); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, in.OutputIdx); err != nil {
			return err
		}
		if err := writeBytes(w, in.Signature); err != nil {
			return err
		}
		if err := writeBytes(w, in.PublicKey); err != nil {
			return err
		}
	}
	if err := writeVarBytesList(w, len(t.Outputs)); err != nil {
		return err
	}
	for _, out := range t.Outputs {
		if err := binary.Write(w, binary.LittleEndian, out.Amount); err != nil {
			return err
		}
		if err := writeBytes(w, out.LockingScript); err != nil {
			return err
		}
	}
	return binary.Write(w, binary.LittleEndian, t.LockTime)
}
func DeserializeTransaction(b []byte) (Transaction, error) {
	return ReadTransaction(bytes.NewReader(b))
}
func ReadTransaction(r io.Reader) (Transaction, error) {
	var t Transaction
	if err := binary.Read(r, binary.LittleEndian, &t.Version); err != nil {
		return t, err
	}
	inputs, err := readCount(r)
	if err != nil {
		return t, err
	}
	if inputs > 10000 {
		return t, fmt.Errorf("too many inputs: %d", inputs)
	}
	t.Inputs = make([]Input, inputs)
	for i := range t.Inputs {
		if _, err := io.ReadFull(r, t.Inputs[i].PrevTxHash[:]); err != nil {
			return t, err
		}
		if err := binary.Read(r, binary.LittleEndian, &t.Inputs[i].OutputIdx); err != nil {
			return t, err
		}
		if t.Inputs[i].Signature, err = readBytes(r); err != nil {
			return t, err
		}
		if t.Inputs[i].PublicKey, err = readBytes(r); err != nil {
			return t, err
		}
	}
	outputs, err := readCount(r)
	if err != nil {
		return t, err
	}
	if outputs > 10000 {
		return t, fmt.Errorf("too many outputs: %d", outputs)
	}
	t.Outputs = make([]Output, outputs)
	for i := range t.Outputs {
		if err := binary.Read(r, binary.LittleEndian, &t.Outputs[i].Amount); err != nil {
			return t, err
		}
		if t.Outputs[i].LockingScript, err = readBytes(r); err != nil {
			return t, err
		}
	}
	return t, binary.Read(r, binary.LittleEndian, &t.LockTime)
}
func writeVarBytesList(w io.Writer, n int) error {
	return binary.Write(w, binary.LittleEndian, uint64(n))
}
func readCount(r io.Reader) (int, error) {
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return 0, err
	}
	if n > 1_000_000 {
		return 0, fmt.Errorf("count too large: %d", n)
	}
	return int(n), nil
}
func writeBytes(w io.Writer, b []byte) error {
	if err := binary.Write(w, binary.LittleEndian, uint64(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}
func readBytes(r io.Reader) ([]byte, error) {
	var n uint64
	if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
		return nil, err
	}
	if n > 1_000_000 {
		return nil, fmt.Errorf("byte field too large: %d", n)
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}
