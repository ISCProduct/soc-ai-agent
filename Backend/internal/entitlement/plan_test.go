package entitlement

import (
	"testing"
	"time"
)

func TestCan_FreeRejectsExport(t *testing.T) {
	if Can(PlanFree, FeatureExport) {
		t.Fatal("free は export 不可")
	}
	if !Can(PlanPro, FeatureExport) {
		t.Fatal("pro は export 可")
	}
}

func TestCan_UnknownPlanFailClosed(t *testing.T) {
	if Can(PlanID("unknown"), FeatureMatching) {
		t.Fatal("未知プランは拒否")
	}
}

func TestCan_AppliedToAcceptedNotRelevant(t *testing.T) {
	if !Can(PlanFree, FeatureInterview) {
		t.Fatal("free でも面接は可（回数上限は #612）")
	}
}

// #985: 組織のplan/contract_end_dateから正しいプランが決定されることを確認する。
func TestPlanForOrganization(t *testing.T) {
	future := time.Now().Add(24 * time.Hour)
	past := time.Now().Add(-24 * time.Hour)

	cases := []struct {
		name            string
		planStr         string
		contractEndDate *time.Time
		want            PlanID
	}{
		{"standard, no contract end date", "standard", nil, PlanStandard},
		{"pro, contract not yet expired", "pro", &future, PlanPro},
		{"pro, but contract expired -> Free", "pro", &past, PlanFree},
		{"unknown plan string -> Free", "enterprise", nil, PlanFree},
		{"empty plan string -> Free", "", nil, PlanFree},
		{"case-insensitive", "STANDARD", nil, PlanStandard},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PlanForOrganization(c.planStr, c.contractEndDate)
			if got != c.want {
				t.Errorf("PlanForOrganization(%q, %v) = %v, want %v", c.planStr, c.contractEndDate, got, c.want)
			}
		})
	}
}
