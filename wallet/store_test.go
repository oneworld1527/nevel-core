package wallet

import "testing"

func TestEncryptedWalletRoundTripAndSeedRestore(t *testing.T) {
	seed, err := NewSeedPhrase(12)
	if err != nil {
		t.Fatal(err)
	}
	w, err := NewFromSeedPhrase(seed, "rnevel")
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir() + "/wallet.json"
	if err := SaveEncryptedWithSeed(path, w, seed, "passphrase"); err != nil {
		t.Fatal(err)
	}
	loaded, loadedSeed, err := LoadEncrypted(path, "passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Address != w.Address || loadedSeed != seed {
		t.Fatalf("wallet restore mismatch")
	}
	if _, _, err := LoadEncrypted(path, "wrong"); err == nil {
		t.Fatal("expected wrong passphrase failure")
	}
}
