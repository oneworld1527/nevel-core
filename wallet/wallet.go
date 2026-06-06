package wallet

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"math/big"

	"github.com/nevel/nevel-core/consensus"
	txpkg "github.com/nevel/nevel-core/tx"
)

type Wallet struct {
	PrivateKey *ecdsa.PrivateKey
	PublicKey  []byte
	Address    string
	Prefix     string
}

func New(prefix string) (*Wallet, error) {
	priv, err := ecdsa.GenerateKey(secp256k1, rand.Reader)
	if err != nil {
		return nil, err
	}
	return FromPrivateKey(priv, prefix)
}

func FromHexPrivateKey(h, prefix string) (*Wallet, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, err
	}
	priv := new(ecdsa.PrivateKey)
	priv.Curve = secp256k1
	priv.D = new(big.Int).SetBytes(b)
	priv.X, priv.Y = priv.Curve.ScalarBaseMult(b)
	return FromPrivateKey(priv, prefix)
}

func FromPrivateKey(priv *ecdsa.PrivateKey, prefix string) (*Wallet, error) {
	pub := marshalCompressed(priv.X, priv.Y)
	addr, err := AddressFromPublicKey(pub, prefix)
	if err != nil {
		return nil, err
	}
	return &Wallet{PrivateKey: priv, PublicKey: pub, Address: addr, Prefix: prefix}, nil
}

func PrivateKeyHex(priv *ecdsa.PrivateKey) string { return hex.EncodeToString(priv.D.Bytes()) }

func Hash160(b []byte) []byte {
	h := sha256.Sum256(b)
	return ripemd160Digest(h[:])
}

func AddressFromPublicKey(pub []byte, prefix string) (string, error) {
	fiveBit, err := convertBits(Hash160(pub), 8, 5, true)
	if err != nil {
		return "", err
	}
	return bech32Encode(prefix, fiveBit)
}

func DecodeAddress(addr, expectedPrefix string) ([]byte, error) {
	hrp, data, err := bech32Decode(addr)
	if err != nil {
		return nil, err
	}
	if hrp != expectedPrefix {
		return nil, errors.New("wrong address prefix")
	}
	return convertBits(data, 5, 8, false)
}

func LockingScriptForAddress(addr, prefix string) ([]byte, error) {
	h, err := DecodeAddress(addr, prefix)
	if err != nil {
		return nil, err
	}
	return consensus.ScriptForPubKeyHash(h), nil
}

func SignatureHash(t txpkg.Transaction, inputIndex int, script []byte) [32]byte {
	copyTx := t
	copyTx.Inputs = append([]txpkg.Input(nil), t.Inputs...)
	for i := range copyTx.Inputs {
		copyTx.Inputs[i].Signature = nil
		copyTx.Inputs[i].PublicKey = nil
	}
	if inputIndex >= 0 && inputIndex < len(copyTx.Inputs) {
		copyTx.Inputs[inputIndex].Signature = script
	}
	h := sha256.Sum256(txpkg.SerializeTransaction(copyTx))
	return sha256.Sum256(h[:])
}

func (w *Wallet) SignInput(t *txpkg.Transaction, inputIndex int, script []byte) error {
	if inputIndex < 0 || inputIndex >= len(t.Inputs) {
		return errors.New("input out of range")
	}
	h := SignatureHash(*t, inputIndex, script)
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, h[:])
	if err != nil {
		return err
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	t.Inputs[inputIndex].Signature = sig
	t.Inputs[inputIndex].PublicKey = w.PublicKey
	return nil
}

type Verifier struct{}

func (Verifier) VerifyTransactionInput(t txpkg.Transaction, inputIndex int, script []byte) bool {
	if inputIndex < 0 || inputIndex >= len(t.Inputs) {
		return false
	}
	want, ok := consensus.ExtractPubKeyHash(script)
	if !ok {
		return false
	}
	in := t.Inputs[inputIndex]
	if got := Hash160(in.PublicKey); string(got) != string(want) {
		return false
	}
	x, y := unmarshalCompressed(in.PublicKey)
	if x == nil {
		return false
	}
	if len(in.Signature) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(in.Signature[:32])
	s := new(big.Int).SetBytes(in.Signature[32:])
	h := SignatureHash(t, inputIndex, script)
	return ecdsa.Verify(&ecdsa.PublicKey{Curve: secp256k1, X: x, Y: y}, h[:], r, s)
}

func marshalCompressed(x, y *big.Int) []byte {
	out := make([]byte, 33)
	if y.Bit(0) == 0 {
		out[0] = 0x02
	} else {
		out[0] = 0x03
	}
	x.FillBytes(out[1:])
	return out
}

func unmarshalCompressed(pub []byte) (*big.Int, *big.Int) {
	if len(pub) != 33 || (pub[0] != 0x02 && pub[0] != 0x03) {
		return nil, nil
	}
	x := new(big.Int).SetBytes(pub[1:])
	if x.Cmp(secp256k1.Params().P) >= 0 {
		return nil, nil
	}
	y2 := new(big.Int).Mul(x, x)
	y2.Mul(y2, x)
	y2.Add(y2, big.NewInt(7))
	y2.Mod(y2, secp256k1.Params().P)
	exponent := new(big.Int).Add(secp256k1.Params().P, big.NewInt(1))
	exponent.Rsh(exponent, 2)
	y := new(big.Int).Exp(y2, exponent, secp256k1.Params().P)
	if y.Bit(0) != uint(pub[0]&1) {
		y.Sub(secp256k1.Params().P, y)
	}
	if !secp256k1.IsOnCurve(x, y) {
		return nil, nil
	}
	return x, y
}
