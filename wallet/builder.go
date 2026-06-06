package wallet

import (
	"errors"

	"github.com/nevel/nevel-core/consensus"
	txpkg "github.com/nevel/nevel-core/tx"
)

type SpendableUTXO struct{ UTXO consensus.UTXO }

type UTXOProvider interface {
	ListUTXOsByScript(script []byte) ([]consensus.UTXO, error)
}

func BuildTransaction(w *Wallet, provider UTXOProvider, toAddress string, amount, fee uint64) (txpkg.Transaction, error) {
	if amount == 0 {
		return txpkg.Transaction{}, errors.New("amount must be greater than zero")
	}
	if ^uint64(0)-amount < fee {
		return txpkg.Transaction{}, txpkg.ErrInflation
	}
	fromScript, err := LockingScriptForAddress(w.Address, w.Prefix)
	if err != nil {
		return txpkg.Transaction{}, err
	}
	toScript, err := LockingScriptForAddress(toAddress, w.Prefix)
	if err != nil {
		return txpkg.Transaction{}, err
	}
	utxos, err := provider.ListUTXOsByScript(fromScript)
	if err != nil {
		return txpkg.Transaction{}, err
	}
	need := amount + fee
	var selected []consensus.UTXO
	var total uint64
	for _, u := range utxos {
		selected = append(selected, u)
		if ^uint64(0)-total < u.Amount {
			return txpkg.Transaction{}, txpkg.ErrInflation
		}
		total += u.Amount
		if total >= need {
			break
		}
	}
	if total < need {
		return txpkg.Transaction{}, errors.New("insufficient funds")
	}
	tr := txpkg.Transaction{Version: 1}
	for _, u := range selected {
		tr.Inputs = append(tr.Inputs, txpkg.Input{PrevTxHash: u.TxHash, OutputIdx: u.OutputIdx})
	}
	tr.Outputs = append(tr.Outputs, txpkg.Output{Amount: amount, LockingScript: toScript})
	change := total - need
	if change > 0 {
		tr.Outputs = append(tr.Outputs, txpkg.Output{Amount: change, LockingScript: fromScript})
	}
	for i := range tr.Inputs {
		if err := w.SignInput(&tr, i, fromScript); err != nil {
			return txpkg.Transaction{}, err
		}
	}
	return tr, nil
}
