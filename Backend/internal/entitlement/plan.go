package entitlement

import (
	"os"
	"strings"
	"time"
)

type PlanID string

const (
	PlanFree     PlanID = "free"
	PlanStandard PlanID = "standard"
	PlanPro      PlanID = "pro"
)

type Feature string

const (
	FeatureMatching     Feature = "matching"
	FeatureInterview    Feature = "interview"
	FeatureResume       Feature = "resume"
	FeatureCompanyGraph Feature = "company_graph"
	FeatureAdmin        Feature = "admin"
	FeatureExport       Feature = "export"
)

// matrix はプラン×機能。未契約は Free。
// ponytail: Stripe 契約は #612。DEFAULT_PLAN 未設定時は pro（CEATEC デモを壊さない）。
var matrix = map[PlanID]map[Feature]bool{
	PlanFree: {
		FeatureMatching: true, FeatureInterview: true, FeatureResume: true,
		FeatureCompanyGraph: false, FeatureAdmin: false, FeatureExport: false,
	},
	PlanStandard: {
		FeatureMatching: true, FeatureInterview: true, FeatureResume: true,
		FeatureCompanyGraph: true, FeatureAdmin: true, FeatureExport: false,
	},
	PlanPro: {
		FeatureMatching: true, FeatureInterview: true, FeatureResume: true,
		FeatureCompanyGraph: true, FeatureAdmin: true, FeatureExport: true,
	},
}

func CurrentPlan() PlanID {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DEFAULT_PLAN"))) {
	case "free":
		return PlanFree
	case "standard":
		return PlanStandard
	default:
		return PlanPro
	}
}

// PlanForOrganization は組織のplan文字列と契約終了日から適用プランを決定する(#985)。
// 契約終了日が過去の場合、またはplan文字列が不正/未設定の場合は安全側のFreeにフォールバックする
// (グローバル環境変数DEFAULT_PLAN基準だと、契約が切れた組織でも機能が使え続けてしまっていた)。
func PlanForOrganization(planStr string, contractEndDate *time.Time) PlanID {
	if contractEndDate != nil && contractEndDate.Before(time.Now()) {
		return PlanFree
	}
	switch strings.ToLower(strings.TrimSpace(planStr)) {
	case "standard":
		return PlanStandard
	case "pro":
		return PlanPro
	default:
		return PlanFree
	}
}

func Can(plan PlanID, feature Feature) bool {
	features, ok := matrix[plan]
	if !ok {
		return false
	}
	return features[feature]
}

func FeaturesFor(plan PlanID) map[Feature]bool {
	src := matrix[plan]
	out := make(map[Feature]bool, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}
