package companyfields

import "testing"

func TestResolve_IT(t *testing.T) {
	cases := []string{"IT・ソフトウェア", "ソフトウェア開発", "情報通信業", "Webサービス"}
	for _, industry := range cases {
		p := Resolve(industry)
		if p.ID != ProfileIT {
			t.Fatalf("industry=%q want it, got %s", industry, p.ID)
		}
		if !p.RequireTechForPublish || !p.TechAspectEnabled {
			t.Fatalf("industry=%q should require/show tech", industry)
		}
	}
}

func TestResolve_NonTech(t *testing.T) {
	cases := []struct {
		industry string
		want     ProfileID
	}{
		{"金融・保険業", ProfileFinance},
		{"銀行", ProfileFinance},
		{"教育・学習支援業", ProfileEducation},
		{"医療・福祉", ProfileHealthcare},
		{"コンサルティング", ProfileConsulting},
		{"製造業", ProfileManufacturing},
		{"", ProfileGeneral},
		{"小売業", ProfileGeneral},
	}
	for _, tc := range cases {
		p := Resolve(tc.industry)
		if p.ID != tc.want {
			t.Fatalf("industry=%q want %s, got %s", tc.industry, tc.want, p.ID)
		}
		if p.ID != ProfileManufacturing && p.RequireTechForPublish {
			t.Fatalf("industry=%q should not require tech", tc.industry)
		}
	}
}

func TestRequiresTechAndTechAspectEnabled(t *testing.T) {
	if !RequiresTech("IT・ソフトウェア") || !TechAspectEnabled("IT・ソフトウェア") {
		t.Fatal("IT should require and enable tech")
	}
	if RequiresTech("製造業") {
		t.Fatal("manufacturing should not require tech for publish")
	}
	if !TechAspectEnabled("製造業") {
		t.Fatal("manufacturing should still show tech aspect")
	}
	if RequiresTech("金融・保険業") || TechAspectEnabled("金融・保険業") {
		t.Fatal("finance should neither require nor enable tech")
	}
	if RequiresTech("") || TechAspectEnabled("") {
		t.Fatal("empty industry (general) should not require/enable tech")
	}
}

func TestTechEmptyStepStatus(t *testing.T) {
	status, detail, asErr := TechEmptyStepStatus("IT・ソフトウェア")
	if status != "empty" || detail != "no_tech_stack" || !asErr {
		t.Fatalf("IT empty: status=%s detail=%s asErr=%v", status, detail, asErr)
	}

	status, detail, asErr = TechEmptyStepStatus("製造業")
	if status != "skipped" || detail != "optional_empty" || asErr {
		t.Fatalf("manufacturing empty: status=%s detail=%s asErr=%v", status, detail, asErr)
	}

	status, detail, asErr = TechEmptyStepStatus("金融・保険業")
	if status != "skipped" || detail != "optional_empty" || asErr {
		t.Fatalf("finance empty: status=%s detail=%s asErr=%v", status, detail, asErr)
	}
}

func TestTechRequiredIndustrySQL_NonEmpty(t *testing.T) {
	cond, args := TechRequiredIndustrySQL("industry")
	if cond == "" || len(args) == 0 {
		t.Fatal("expected non-empty SQL for tech-required industries")
	}
}
