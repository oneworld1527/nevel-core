package wallet

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestHash160UsesRIPEMD160OfSHA256(t *testing.T) {
	got := hex.EncodeToString(Hash160(nil))
	want := "b472a266d0bd89c13706a4132ccfb16f7c3b9fcb"
	if got != want {
		t.Fatalf("Hash160(nil)=%s want %s", got, want)
	}
}

func TestAddressIsBech32WithChecksum(t *testing.T) {
	w, err := New("rnevel")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(w.Address, "rnevel1") {
		t.Fatalf("address %q missing regtest hrp", w.Address)
	}
	h, err := DecodeAddress(w.Address, "rnevel")
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 20 {
		t.Fatalf("decoded pubkey hash len=%d", len(h))
	}
	bad := w.Address[:len(w.Address)-1] + "x"
	if _, err := DecodeAddress(bad, "rnevel"); err == nil {
		t.Fatal("expected checksum failure for mutated address")
	}
}

func TestSecp256k1GeneratorIsOnCurve(t *testing.T) {
	if !secp256k1.IsOnCurve(secp256k1.Params().Gx, secp256k1.Params().Gy) {
		t.Fatal("secp256k1 generator is not on curve")
	}
}
