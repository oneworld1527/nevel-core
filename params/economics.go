package params

import "errors"

var ErrSupplyScheduleExceedsMax = errors.New("supply schedule exceeds maximum supply")

type SupplyAudit struct {
	MaxSupply       uint64 `json:"maxSupply"`
	InitialReward   uint64 `json:"initialReward"`
	HalvingInterval uint64 `json:"halvingInterval"`
	TotalSubsidy    uint64 `json:"totalSubsidy"`
	Pass            bool   `json:"pass"`
}

func AuditSupplySchedule(maxHeight uint64) (SupplyAudit, error) {
	report := SupplyAudit{MaxSupply: MaxSupply, InitialReward: InitialReward, HalvingInterval: HalvingInterval}
	var total uint64
	for height := uint64(0); height <= maxHeight; height++ {
		reward := InitialReward >> (height / HalvingInterval)
		if height/HalvingInterval >= 64 {
			reward = 0
		}
		if ^uint64(0)-total < reward || total+reward > MaxSupply {
			report.TotalSubsidy = total
			return report, ErrSupplyScheduleExceedsMax
		}
		total += reward
		if reward == 0 {
			break
		}
	}
	report.TotalSubsidy = total
	report.Pass = total <= MaxSupply
	return report, nil
}
