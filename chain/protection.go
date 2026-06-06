package chain

import (
	"errors"
	"math/big"
	"time"

	"github.com/nevel/nevel-core/blockchain"
	"github.com/nevel/nevel-core/consensus"
	"github.com/nevel/nevel-core/params"
)

var (
	ErrReorgTooDeep      = errors.New("reorg exceeds configured depth")
	ErrInsufficientWork  = errors.New("candidate chain does not have more accumulated work")
	ErrDifficultyAnomaly = errors.New("difficulty change exceeds consensus bounds")
)

type ReorgPolicy struct {
	MaxDepth        uint64
	RequireMoreWork bool
}

type ForkChoice struct {
	CurrentWork   *big.Int
	CandidateWork *big.Int
	ReorgDepth    uint64
	Allowed       bool
	Reason        string
}

type HealthThresholds struct {
	MaxTipAge       time.Duration
	MaxReorgDepth   uint64
	MinPeerCount    int
	MinHashrateWork *big.Int
}

type HealthReport struct {
	TipHeight     uint64   `json:"tipHeight"`
	TipAgeSeconds int64    `json:"tipAgeSeconds"`
	PeerCount     int      `json:"peerCount"`
	Alerts        []string `json:"alerts"`
}

func EvaluateForkChoice(currentWork, candidateWork *big.Int, reorgDepth uint64, policy ReorgPolicy) (ForkChoice, error) {
	if currentWork == nil {
		currentWork = big.NewInt(0)
	}
	if candidateWork == nil {
		candidateWork = big.NewInt(0)
	}
	choice := ForkChoice{CurrentWork: new(big.Int).Set(currentWork), CandidateWork: new(big.Int).Set(candidateWork), ReorgDepth: reorgDepth}
	if policy.MaxDepth > 0 && reorgDepth > policy.MaxDepth {
		choice.Reason = ErrReorgTooDeep.Error()
		return choice, ErrReorgTooDeep
	}
	if policy.RequireMoreWork && candidateWork.Cmp(currentWork) <= 0 {
		choice.Reason = ErrInsufficientWork.Error()
		return choice, ErrInsufficientWork
	}
	choice.Allowed = true
	return choice, nil
}

func ValidateDifficultyTransition(prev blockchain.BlockHeader, nextBits uint32, net params.Network, headerByHeight consensus.HeaderByHeight) error {
	expected, err := consensus.ExpectedBits(prev, net, headerByHeight)
	if err != nil {
		return err
	}
	if nextBits != expected {
		return ErrDifficultyAnomaly
	}
	return nil
}

func AssessHealth(tip Tip, peers int, thresholds HealthThresholds, now time.Time) HealthReport {
	report := HealthReport{TipHeight: tip.Height, PeerCount: peers}
	age := now.Sub(time.Unix(tip.Timestamp, 0))
	if age < 0 {
		age = 0
	}
	report.TipAgeSeconds = int64(age.Seconds())
	if thresholds.MaxTipAge > 0 && age > thresholds.MaxTipAge {
		report.Alerts = append(report.Alerts, "stale_tip")
	}
	if thresholds.MinPeerCount > 0 && peers < thresholds.MinPeerCount {
		report.Alerts = append(report.Alerts, "low_peer_count")
	}
	return report
}
