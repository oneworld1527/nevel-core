package wallet

import (
	"errors"

	txpkg "github.com/nevel/nevel-core/tx"
)

type HardwareSigner interface {
	PublicKey() []byte
	Address(prefix string) (string, error)
	SignInput(t txpkg.Transaction, inputIndex int, script []byte) ([]byte, error)
}

type HardwareWallet struct {
	Signer HardwareSigner
	Prefix string
}

func (h HardwareWallet) Address() (string, error) {
	if h.Signer == nil {
		return "", errors.New("hardware signer unavailable")
	}
	return h.Signer.Address(h.Prefix)
}
func (h HardwareWallet) Sign(t *txpkg.Transaction, inputIndex int, script []byte) error {
	if h.Signer == nil {
		return errors.New("hardware signer unavailable")
	}
	sig, err := h.Signer.SignInput(*t, inputIndex, script)
	if err != nil {
		return err
	}
	t.Inputs[inputIndex].Signature = sig
	t.Inputs[inputIndex].PublicKey = h.Signer.PublicKey()
	return nil
}
