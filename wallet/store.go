package wallet

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
)

// EncryptedFile is the on-disk format for an encrypted wallet.
// Version 2 uses PBKDF2-SHA256 for key derivation; version 1 used raw SHA-256.
type EncryptedFile struct {
	Version    int    `json:"version"`
	Address    string `json:"address"`
	Prefix     string `json:"prefix"`
	KDF        string `json:"kdf"`
	KDFSalt    string `json:"kdfSalt,omitempty"`
	KDFIter    int    `json:"kdfIter,omitempty"`
	Nonce      string `json:"nonce"`
	Ciphertext string `json:"ciphertext"`
}

type walletPlaintext struct {
	PrivateKeyHex string `json:"privateKeyHex"`
	SeedPhrase    string `json:"seedPhrase,omitempty"`
}

const (
	currentVersion = 2
	pbkdf2Iter     = 100_000
)

// deriveKey derives a 32-byte AES key from passphrase + salt using PBKDF2-SHA256.
func deriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2SHA256([]byte(passphrase), salt, pbkdf2Iter, 32)
}

// SaveEncrypted saves a wallet to an encrypted file using PBKDF2 key derivation.
func SaveEncrypted(path string, w *Wallet, passphrase string) error {
	return SaveEncryptedWithSeed(path, w, "", passphrase)
}

// SaveEncryptedWithSeed saves a wallet and its seed phrase to an encrypted file.
func SaveEncryptedWithSeed(path string, w *Wallet, seedPhrase, passphrase string) error {
	if passphrase == "" {
		return errors.New("passphrase required")
	}
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	plain, err := json.Marshal(walletPlaintext{PrivateKeyHex: PrivateKeyHex(w.PrivateKey), SeedPhrase: seedPhrase})
	if err != nil {
		return err
	}
	encrypted := gcm.Seal(nil, nonce, plain, []byte(w.Address))
	file := EncryptedFile{
		Version:    currentVersion,
		Address:    w.Address,
		Prefix:     w.Prefix,
		KDF:        "pbkdf2-sha256",
		KDFSalt:    hex.EncodeToString(salt),
		KDFIter:    pbkdf2Iter,
		Nonce:      hex.EncodeToString(nonce),
		Ciphertext: hex.EncodeToString(encrypted),
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadEncrypted loads a wallet from an encrypted file.
// It supports both version 1 (raw SHA-256) and version 2 (PBKDF2) files.
func LoadEncrypted(path, passphrase string) (*Wallet, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	var file EncryptedFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, "", err
	}

	var key []byte
	switch file.Version {
	case 1:
		// Legacy: raw SHA-256 of passphrase (no salt).
		k := sha256.Sum256([]byte(passphrase))
		key = k[:]
	case 2:
		salt, err := hex.DecodeString(file.KDFSalt)
		if err != nil {
			return nil, "", err
		}
		iters := file.KDFIter
		if iters <= 0 {
			iters = pbkdf2Iter
		}
		key = pbkdf2SHA256([]byte(passphrase), salt, iters, 32)
	default:
		return nil, "", errors.New("unsupported wallet version")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, "", err
	}
	nonce, err := hex.DecodeString(file.Nonce)
	if err != nil {
		return nil, "", err
	}
	ciphertext, err := hex.DecodeString(file.Ciphertext)
	if err != nil {
		return nil, "", err
	}
	plain, err := gcm.Open(nil, nonce, ciphertext, []byte(file.Address))
	if err != nil {
		return nil, "", errors.New("wrong passphrase or corrupt wallet")
	}
	var decoded walletPlaintext
	if err := json.Unmarshal(plain, &decoded); err != nil {
		return nil, "", err
	}
	w, err := FromHexPrivateKey(decoded.PrivateKeyHex, file.Prefix)
	if err != nil {
		return nil, "", err
	}
	if w.Address != file.Address {
		return nil, "", errors.New("wallet address mismatch")
	}
	return w, decoded.SeedPhrase, nil
}

// seedWords is the built-in mnemonic word list.
var seedWords = []string{
	"able", "about", "above", "absent", "absorb", "access", "acid", "across",
	"act", "adapt", "add", "adjust", "admit", "adult", "agent", "ahead",
	"alarm", "album", "alert", "alien", "align", "allow", "alone", "alpha",
	"always", "amber", "anchor", "angle", "apple", "april", "arena", "arrow",
	"asset", "atom", "audit", "avoid", "badge", "balance", "basic", "beach",
	"because", "before", "binary", "bonus", "brave", "bridge", "budget",
	"cable", "camera", "carbon", "chain", "circle", "client", "coin", "core",
	"credit", "crypto", "dash", "delta", "digital", "dollar", "double",
	"dream", "eagle", "earth", "elite", "energy", "engine", "equal", "ethic",
	"fabric", "faith", "fiber", "final", "future", "galaxy", "genesis",
	"global", "gold", "green", "guard", "hash", "honest", "index", "input",
	"jewel", "kernel", "layer", "ledger", "limit", "main", "market", "matrix",
	"memory", "merkle", "miner", "mobile", "money", "native", "network",
	"nevel", "node", "nonce", "output", "packet", "payment", "peer", "proof",
	"public", "quantum", "reward", "script", "secure", "seed", "signal",
	"silver", "supply", "target", "token", "trust", "valid", "wallet",
	"work", "zero",
}

// NewSeedPhrase generates a random mnemonic of the given word count.
func NewSeedPhrase(words int) (string, error) {
	if words <= 0 {
		words = 24
	}
	out := make([]string, words)
	b := make([]byte, words)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i, v := range b {
		out[i] = seedWords[int(v)%len(seedWords)]
	}
	return strings.Join(out, " "), nil
}

// NewFromSeedPhrase derives a deterministic wallet from a seed phrase using
// PBKDF2-SHA256 with a fixed "nevel-seed" salt.
func NewFromSeedPhrase(seed, prefix string) (*Wallet, error) {
	key := pbkdf2SHA256([]byte(seed), []byte("nevel-seed"), pbkdf2Iter, 32)
	return FromHexPrivateKey(hex.EncodeToString(key), prefix)
}
