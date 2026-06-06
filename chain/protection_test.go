package chain

import (
	"errors"
	"math/big"
	"testing"
	"time"
)

func TestEvaluateForkChoiceRequiresMoreWork(t *testing.T) {
	_, err := EvaluateForkChoice(big.NewInt(100), big.NewInt(100), 1, ReorgPolicy{MaxDepth: 10, RequireMoreWork: true})
	if !errors.Is(err, ErrInsufficientWork) {
		t.Fatalf("expected insufficient work error, got %v", err)
	}
	choice, err := EvaluateForkChoice(big.NewInt(100), big.NewInt(101), 1, ReorgPolicy{MaxDepth: 10, RequireMoreWork: true})
	if err != nil || !choice.Allowed {
		t.Fatalf("expected better-work fork to be allowed, choice=%+v err=%v", choice, err)
	}
}

func TestEvaluateForkChoiceRejectsDeepReorg(t *testing.T) {
	_, err := EvaluateForkChoice(big.NewInt(100), big.NewInt(200), 11, ReorgPolicy{MaxDepth: 10, RequireMoreWork: true})
	if !errors.Is(err, ErrReorgTooDeep) {
		t.Fatalf("expected deep reorg error, got %v", err)
	}
}

func TestAssessHealthAlerts(t *testing.T) {
	now := time.Unix(1000, 0)
	report := AssessHealth(Tip{Height: 7, Timestamp: 1}, 0, HealthThresholds{MaxTipAge: time.Minute, MinPeerCount: 1}, now)
	if len(report.Alerts) != 2 {
		t.Fatalf("expected stale tip and low peer alerts, got %#v", report.Alerts)
	}
}
