package wallet

import "testing"

func TestAuditSeedPhrase(t *testing.T) {
	seed := "able about above absent absorb access acid across act adapt add adjust admit adult agent ahead alarm album alert alien align allow alone alpha"
	report, err := AuditSeedPhrase(seed)
	if err != nil {
		t.Fatalf("valid seed failed audit: %v", err)
	}
	if !report.Valid || report.Words != 24 || report.EstimatedEntropy != 192 {
		t.Fatalf("unexpected audit report: %+v", report)
	}
}

func TestAuditSeedPhraseRejectsUnknownWord(t *testing.T) {
	_, err := AuditSeedPhrase("able about above absent absorb access acid across act adapt add zzz")
	if err == nil {
		t.Fatal("expected unknown word to fail")
	}
}
