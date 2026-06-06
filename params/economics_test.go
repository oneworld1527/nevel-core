package params

import "testing"

func TestAuditSupplySchedule(t *testing.T) {
	report, err := AuditSupplySchedule(HalvingInterval * 64)
	if err != nil {
		t.Fatalf("supply schedule audit failed: %v", err)
	}
	if !report.Pass || report.TotalSubsidy > MaxSupply {
		t.Fatalf("unexpected supply report: %+v", report)
	}
}
