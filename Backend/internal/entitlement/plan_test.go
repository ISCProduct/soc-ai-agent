package entitlement

import "testing"

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
