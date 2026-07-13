package services

import (
	"errors"
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

type briefRepoStub struct {
	byID map[uint]*models.Company
}

func (s *briefRepoStub) FindByID(id uint) (*models.Company, error) {
	if c, ok := s.byID[id]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}

func (s *briefRepoStub) FindByName(name string) (*models.Company, error) {
	for _, c := range s.byID {
		if c.Name == name {
			return c, nil
		}
	}
	return nil, errors.New("not found")
}

func (s *briefRepoStub) GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error) {
	return nil, errors.New("not found")
}

func TestResolveCompanyInfo_PrefersDBBrief(t *testing.T) {
	repo := &briefRepoStub{
		byID: map[uint]*models.Company{
			42: {
				ID:           42,
				Name:         "中小SIテスト株式会社",
				MainBusiness: "受託開発・SES",
				Culture:      "チームワーク重視",
			},
		},
	}
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	svc.SetCompanyRepo(repo)

	got := svc.resolveCompanyInfo(42, "中小SIテスト株式会社", "クライアント文面")
	assert.Contains(t, got, "受託開発・SES")
	assert.NotContains(t, got, "クライアント文面")

	empty := svc.resolveCompanyInfo(0, "存在しない会社XYZ", "クライアント文面")
	assert.Equal(t, "クライアント文面", empty)

	got2 := svc.resolveCompanyInfo(42, "", "")
	assert.Contains(t, got2, "中小SIテスト株式会社")
}

func TestResolveCompanyID_ByNameWhenZero(t *testing.T) {
	repo := &briefRepoStub{
		byID: map[uint]*models.Company{
			42: {
				ID:   42,
				Name: "中小SIテスト株式会社",
			},
		},
	}
	svc := NewInterviewService(nil, nil, nil, nil, nil, nil, nil)
	svc.SetCompanyRepo(repo)

	assert.Equal(t, uint(42), svc.resolveCompanyID(0, "中小SIテスト株式会社"))
	assert.Equal(t, uint(42), svc.resolveCompanyID(42, "別名"))
	assert.Equal(t, uint(0), svc.resolveCompanyID(0, "存在しない会社XYZ"))
	assert.Equal(t, uint(0), svc.resolveCompanyID(0, ""))
}
