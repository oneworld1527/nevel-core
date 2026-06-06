package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/params"
	txpkg "github.com/nevel/nevel-core/tx"
)

var (
	ErrInvalidProofOfWork = errors.New("invalid proof of work")
	ErrInvalidMerkleRoot  = errors.New("invalid merkle root")
	ErrInvalidCoinbase    = errors.New("invalid coinbase")
	ErrInvalidSignature   = errors.New("invalid signature")
	ErrMissingUTXO        = errors.New("missing utxo")
	ErrDoubleSpend        = errors.New("double spend")
	ErrBadTimestamp       = errors.New("bad timestamp")
	ErrImmatureCoinbase   = errors.New("coinbase output is not mature")
	ErrBadDifficulty      = errors.New("bad difficulty target")
)

type UTXO struct {
	TxHash    [32]byte
	OutputIdx uint32
	Amount    uint64
	Script    []byte
	Height    uint64
	Coinbase  bool
}
type UTXOReader interface {
	Get(hash [32]byte, idx uint32) (UTXO, error)
}
type SignatureVerifier interface {
	VerifyTransactionInput(t txpkg.Transaction, inputIndex int, script []byte) bool
}
type HeaderByHeight func(height uint64) (blockchain.BlockHeader, error)

type Rules struct {
	Network       params.Network
	Verify        SignatureVerifier
	MinFeePerByte uint64
}

func BlockReward(height uint64) uint64 {
	halvings := height / params.HalvingInterval
	if halvings >= 64 {
		return 0
	}
	return params.InitialReward >> halvings
}
func Retarget(oldTarget *big.Int, actualTime, expectedTime int64) *big.Int {
	min := expectedTime / 4
	max := expectedTime * 4
	if actualTime < min {
		actualTime = min
	}
	if actualTime > max {
		actualTime = max
	}
	n := new(big.Int).Mul(oldTarget, big.NewInt(actualTime))
	n.Div(n, big.NewInt(expectedTime))
	return n
}
func WorkForBits(bits uint32) *big.Int {
	target := params.CompactToBig(bits)
	if target.Sign() <= 0 {
		return big.NewInt(0)
	}
	denom := new(big.Int).Add(target, big.NewInt(1))
	return new(big.Int).Div(new(big.Int).Lsh(big.NewInt(1), 256), denom)
}
func CheckProofOfWork(h blockchain.BlockHeader, powLimit *big.Int) error {
	target := params.CompactToBig(h.Bits)
	if target.Sign() <= 0 || target.Cmp(powLimit) > 0 {
		return ErrInvalidProofOfWork
	}
	hash := h.Hash()
	if new(big.Int).SetBytes(hash[:]).Cmp(target) > 0 {
		return ErrInvalidProofOfWork
	}
	return nil
}
func ExpectedBits(prev blockchain.BlockHeader, net params.Network, headerByHeight HeaderByHeight) (uint32, error) {
	nextHeight := prev.Height + 1
	if nextHeight%params.DifficultyWindow != 0 || nextHeight == 0 {
		return prev.Bits, nil
	}
	firstHeight := nextHeight - params.DifficultyWindow
	first, err := headerByHeight(firstHeight)
	if err != nil {
		return 0, err
	}
	actual := prev.Timestamp - first.Timestamp
	old := params.CompactToBig(prev.Bits)
	retargeted := Retarget(old, actual, params.ExpectedRetargetSec)
	if retargeted.Cmp(net.PowLimit) > 0 {
		retargeted = new(big.Int).Set(net.PowLimit)
	}
	return params.BigToCompact(retargeted), nil
}
func ValidateBlockHeader(b blockchain.Block, prev *blockchain.BlockHeader, net params.Network) error {
	if b.Header.MerkleRoot != blockchain.BuildMerkleRoot(b.Transactions) {
		return ErrInvalidMerkleRoot
	}
	if err := CheckProofOfWork(b.Header, net.PowLimit); err != nil {
		return err
	}
	if b.Header.Timestamp > time.Now().Add(2*time.Hour).Unix() {
		return ErrBadTimestamp
	}
	if prev != nil {
		if b.Header.Height != prev.Height+1 {
			return fmt.Errorf("bad height")
		}
		if b.Header.PrevHash != prev.Hash() {
			return fmt.Errorf("bad previous hash")
		}
	}
	return nil
}
func ValidateBlockHeaderWithDifficulty(b blockchain.Block, prev *blockchain.BlockHeader, net params.Network, headerByHeight HeaderByHeight) error {
	if err := ValidateBlockHeader(b, prev, net); err != nil {
		return err
	}
	if prev != nil {
		expected, err := ExpectedBits(*prev, net, headerByHeight)
		if err != nil {
			return err
		}
		if b.Header.Bits != expected {
			return ErrBadDifficulty
		}
	}
	return nil
}
func ValidateBlockTransactions(b blockchain.Block, utxos UTXOReader, verifier SignatureVerifier) error {
	return ValidateBlockTransactionsAt(b, utxos, verifier, b.Header.Height)
}
func ValidateBlockTransactionsAt(b blockchain.Block, utxos UTXOReader, verifier SignatureVerifier, spendHeight uint64) error {
	if len(b.Transactions) == 0 || !b.Transactions[0].IsCoinbase() {
		return ErrInvalidCoinbase
	}
	reward := BlockReward(b.Header.Height)
	var fees uint64
	spent := map[string]struct{}{}
	for i, t := range b.Transactions {
		if i == 0 {
			continue
		}
		fee, err := ValidateTransactionAt(t, utxos, verifier, spent, spendHeight)
		if err != nil {
			return err
		}
		if ^uint64(0)-fees < fee {
			return txpkg.ErrInflation
		}
		fees += fee
	}
	var coinbaseOut uint64
	for _, out := range b.Transactions[0].Outputs {
		if ^uint64(0)-coinbaseOut < out.Amount {
			return txpkg.ErrInflation
		}
		coinbaseOut += out.Amount
	}
	if coinbaseOut > reward+fees {
		return ErrInvalidCoinbase
	}
	return nil
}
func ValidateTransaction(t txpkg.Transaction, utxos UTXOReader, verifier SignatureVerifier, pendingSpent map[string]struct{}) (uint64, error) {
	return ValidateTransactionAt(t, utxos, verifier, pendingSpent, ^uint64(0))
}
func ValidateTransactionAt(t txpkg.Transaction, utxos UTXOReader, verifier SignatureVerifier, pendingSpent map[string]struct{}, spendHeight uint64) (uint64, error) {
	if t.IsCoinbase() {
		return 0, ErrInvalidCoinbase
	}
	var inputTotal, outputTotal uint64
	for i, in := range t.Inputs {
		key := OutpointKey(in.PrevTxHash, in.OutputIdx)
		if _, ok := pendingSpent[key]; ok {
			return 0, ErrDoubleSpend
		}
		u, err := utxos.Get(in.PrevTxHash, in.OutputIdx)
		if err != nil {
			return 0, ErrMissingUTXO
		}
		if u.Coinbase && spendHeight != ^uint64(0) && spendHeight < u.Height+params.CoinbaseMaturity {
			return 0, ErrImmatureCoinbase
		}
		if verifier != nil && !verifier.VerifyTransactionInput(t, i, u.Script) {
			return 0, ErrInvalidSignature
		}
		if ^uint64(0)-inputTotal < u.Amount {
			return 0, txpkg.ErrInflation
		}
		inputTotal += u.Amount
		pendingSpent[key] = struct{}{}
	}
	for _, out := range t.Outputs {
		if out.Amount == 0 {
			return 0, txpkg.ErrDustOutput
		}
		if ^uint64(0)-outputTotal < out.Amount {
			return 0, txpkg.ErrInflation
		}
		outputTotal += out.Amount
	}
	if outputTotal > inputTotal {
		return 0, txpkg.ErrInflation
	}
	fee := inputTotal - outputTotal
	if fee < MinFee(t) {
		return 0, txpkg.ErrFeeTooLow
	}
	return fee, nil
}
func MinFee(t txpkg.Transaction) uint64 {
	return uint64(len(txpkg.SerializeTransaction(t))) * params.MinRelayFeePerByte
}
func OutpointKey(hash [32]byte, idx uint32) string { return fmt.Sprintf("%x:%d", hash, idx) }
func ScriptForPubKeyHash(hash []byte) []byte       { return append([]byte("p2pkh:"), hash...) }
func ExtractPubKeyHash(script []byte) ([]byte, bool) {
	prefix := []byte("p2pkh:")
	if !bytes.HasPrefix(script, prefix) {
		return nil, false
	}
	return script[len(prefix):], true
}
