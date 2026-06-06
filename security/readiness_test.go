package security

import (
	"errors"
	"testing"
	"time"
)

func TestValidateReleaseEvidenceRequiresAllControls(t *testing.T) {
	err := ValidateReleaseEvidence(ReleaseEvidence{}, time.Now())
	if !errors.Is(err, ErrReleaseGateFailed) {
		t.Fatalf("expected release gate failure, got %v", err)
	}
}

func TestValidateReleaseEvidencePassesCompletePackage(t *testing.T) {
	now := time.Now()
	err := ValidateReleaseEvidence(ReleaseEvidence{
		IndependentSecurityAudit: AuditEvidence{Auditor: "independent lab", ReportURL: "https://example.invalid/report.pdf", ReportSHA256: "abc", CompletedAt: now.Add(-time.Hour), Scope: []string{"consensus", "p2p", "wallet"}},
		PublicTestnet:            BattleTestEvidence{Network: "testnet", PublicStart: now.Add(-60 * 24 * time.Hour), PublicEnd: now, MinimumDuration: 30 * 24 * time.Hour, SpamDrill: true, MalformedP2PDrill: true, ReorgDrill: true, WalletRestoreDrill: true},
		Controls:                 OperationalControls{P2PHardening: true, DifficultyReorgGuards: true, WalletSeedAudit: true, MiningPoolSecurity: true, DDoSProtection: true, ExchangeMonitoring: true, SupplyEconomicAudit: true},
	}, now)
	if err != nil {
		t.Fatalf("complete evidence should pass: %v", err)
	}
}
