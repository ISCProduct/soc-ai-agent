package company

import (
	"testing"

	"Backend/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestBuildCompanyBrief_FromDBFields(t *testing.T) {
	company := &models.Company{
		Name:         "株式会社サンプルテック",
		Description:  "長い説明は事業欄のフォールバック候補",
		MainBusiness: "BtoB SaaSの開発・提供",
		Industry:     "情報通信業",
		Location:     "東京都",
		Culture:      "フラットな組織文化でチャレンジを推奨する",
		WorkStyle:    "ハイブリッド",
		TechStack:    `["Go","TypeScript"]`,
	}
	profile := &models.CompanyWeightProfile{
		TechnicalOrientation: 90,
		GrowthOrientation:    80,
		TeamworkOrientation:  40,
	}
	brief := BuildCompanyBrief(company, profile)
	assert.Contains(t, brief, "株式会社サンプルテック")
	assert.Contains(t, brief, "BtoB SaaS")
	assert.Contains(t, brief, "技術志向(90)")
	assert.NotContains(t, brief, "チームワーク(40)")
}
