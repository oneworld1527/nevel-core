package tx

import "testing"

func FuzzDeserializeTransaction(f *testing.F) {
	f.Add(SerializeTransaction(Transaction{Version: 1, Outputs: []Output{{Amount: 1, LockingScript: []byte("x")}}}))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DeserializeTransaction(data) })
}
