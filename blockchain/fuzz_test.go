package blockchain

import "testing"

func FuzzDeserializeBlock(f *testing.F) {
	f.Add(SerializeBlock(NewBlock([32]byte{}, 1, 0x207fffff, nil)))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = DeserializeBlock(data) })
}
