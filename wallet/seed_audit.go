package wallet

import (
	"errors"
	"strings"
)

var (
	ErrSeedWordCount = errors.New("seed phrase must contain 12, 18, or 24 words")
	ErrSeedWordlist  = errors.New("seed phrase contains a non NEVEL wordlist word")
)

type SeedAuditReport struct {
	Words            int      `json:"words"`
	Wordlist         string   `json:"wordlist"`
	EstimatedEntropy int      `json:"estimatedEntropy"`
	StandardWarnings []string `json:"standardWarnings,omitempty"`
	Valid            bool     `json:"valid"`
}

func AuditSeedPhrase(seed string) (SeedAuditReport, error) {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(seed)))
	report := SeedAuditReport{Words: len(words), Wordlist: "nevel-v1", EstimatedEntropy: len(words) * 8}
	if len(words) != 12 && len(words) != 18 && len(words) != 24 {
		report.StandardWarnings = append(report.StandardWarnings, ErrSeedWordCount.Error())
		return report, ErrSeedWordCount
	}
	known := map[string]struct{}{}
	for _, w := range seedWords {
		known[w] = struct{}{}
	}
	for _, w := range words {
		if _, ok := known[w]; !ok {
			report.StandardWarnings = append(report.StandardWarnings, ErrSeedWordlist.Error())
			return report, ErrSeedWordlist
		}
	}
	if len(words) == 24 {
		report.StandardWarnings = append(report.StandardWarnings, "NEVEL v1 seeds are audited for internal compatibility but are not BIP-39 mnemonic checksums")
	}
	report.Valid = true
	return report, nil
}
