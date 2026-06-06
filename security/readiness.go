package security

import (
	"errors"
	"fmt"
	"time"
)

var ErrReleaseGateFailed = errors.New("release security gate failed")

type AuditEvidence struct {
	Auditor      string    `json:"auditor"`
	ReportURL    string    `json:"reportUrl"`
	ReportSHA256 string    `json:"reportSha256"`
	CompletedAt  time.Time `json:"completedAt"`
	Scope        []string  `json:"scope"`
	CriticalOpen int       `json:"criticalOpen"`
	HighOpen     int       `json:"highOpen"`
}

type BattleTestEvidence struct {
	Network            string        `json:"network"`
	PublicStart        time.Time     `json:"publicStart"`
	PublicEnd          time.Time     `json:"publicEnd"`
	MinimumDuration    time.Duration `json:"minimumDuration"`
	SpamDrill          bool          `json:"spamDrill"`
	MalformedP2PDrill  bool          `json:"malformedP2PDrill"`
	ReorgDrill         bool          `json:"reorgDrill"`
	WalletRestoreDrill bool          `json:"walletRestoreDrill"`
}

type OperationalControls struct {
	P2PHardening          bool `json:"p2pHardening"`
	DifficultyReorgGuards bool `json:"difficultyReorgGuards"`
	WalletSeedAudit       bool `json:"walletSeedAudit"`
	MiningPoolSecurity    bool `json:"miningPoolSecurity"`
	DDoSProtection        bool `json:"ddosProtection"`
	ExchangeMonitoring    bool `json:"exchangeMonitoring"`
	SupplyEconomicAudit   bool `json:"supplyEconomicAudit"`
}

type ReleaseEvidence struct {
	IndependentSecurityAudit AuditEvidence       `json:"independentSecurityAudit"`
	PublicTestnet            BattleTestEvidence  `json:"publicTestnet"`
	Controls                 OperationalControls `json:"controls"`
}

func ValidateReleaseEvidence(e ReleaseEvidence, now time.Time) error {
	var missing []string
	if e.IndependentSecurityAudit.Auditor == "" || e.IndependentSecurityAudit.ReportURL == "" || e.IndependentSecurityAudit.ReportSHA256 == "" || e.IndependentSecurityAudit.CompletedAt.IsZero() || e.IndependentSecurityAudit.CompletedAt.After(now) {
		missing = append(missing, "independent security audit evidence")
	}
	if e.IndependentSecurityAudit.CriticalOpen > 0 || e.IndependentSecurityAudit.HighOpen > 0 {
		missing = append(missing, "critical/high audit findings resolved")
	}
	if e.PublicTestnet.Network == "" || e.PublicTestnet.PublicStart.IsZero() || e.PublicTestnet.PublicEnd.Sub(e.PublicTestnet.PublicStart) < e.PublicTestnet.MinimumDuration || !e.PublicTestnet.SpamDrill || !e.PublicTestnet.MalformedP2PDrill || !e.PublicTestnet.ReorgDrill || !e.PublicTestnet.WalletRestoreDrill {
		missing = append(missing, "public testnet battle-testing evidence")
	}
	controls := map[string]bool{
		"real P2P hardening":                 e.Controls.P2PHardening,
		"mature difficulty/reorg protection": e.Controls.DifficultyReorgGuards,
		"wallet seed phrase standard audit":  e.Controls.WalletSeedAudit,
		"mining pool/hashrate security":      e.Controls.MiningPoolSecurity,
		"DDoS protection":                    e.Controls.DDoSProtection,
		"exchange-grade monitoring":          e.Controls.ExchangeMonitoring,
		"formal supply/economic audit":       e.Controls.SupplyEconomicAudit,
	}
	for name, ok := range controls {
		if !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: %v", ErrReleaseGateFailed, missing)
	}
	return nil
}
