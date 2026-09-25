package resume

import (
	"context"
	"errors"
	"strings"
	"testing"

	"Backend/internal/models"
)

type briefReaderStub struct {
	company *models.Company
	profile *models.CompanyWeightProfile
	profErr error
}

func (s *briefReaderStub) FindByID(id uint) (*models.Company, error) { return s.company, nil }
func (s *briefReaderStub) FindByName(name string) (*models.Company, error) {
	if s.company == nil {
		return nil, errors.New("not found")
	}
	return s.company, nil
}
func (s *briefReaderStub) GetWeightProfile(companyID uint, jobPositionID *uint) (*models.CompanyWeightProfile, error) {
	if s.profErr != nil {
		return nil, s.profErr
	}
	return s.profile, nil
}

type personaStub struct {
	calls   int
	profile *models.CompanyWeightProfile
	err     error
}

func (p *personaStub) FetchAndSavePersona(ctx context.Context, companyID uint, forceRefresh bool) (*models.CompanyWeightProfile, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.profile, nil
}

// TestLookupCompanyBrief_EnsuresPersona は「求める人材像」が未生成なら1回だけ作ることを検証する（#1124）。
//
// brief の「重視傾向」が企業の求める人材像そのもので、これが無いとレビューは
// 業種・事業内容だけを見た一般論になる。実測では 842社中90社(10.7%)しか持っていなかった。
func TestLookupCompanyBrief_EnsuresPersona(t *testing.T) {
	comp := &models.Company{ID: 7, Name: "株式会社テスト", Industry: "情報通信業"}
	// リーダーシップを最重視するプロファイル
	generated := &models.CompanyWeightProfile{
		CompanyID:             7,
		LeadershipOrientation: 90,
		TechnicalOrientation:  80,
		CommunicationSkill:    70,
		TeamworkOrientation:   40,
	}

	tests := []struct {
		name       string
		existing   *models.CompanyWeightProfile
		persona    *personaStub
		wantCalls  int
		wantInText string
	}{
		{
			name:       "未生成なら1回だけ生成して重視傾向が出る",
			existing:   nil,
			persona:    &personaStub{profile: generated},
			wantCalls:  1,
			wantInText: "重視傾向",
		},
		{
			name:       "既にあれば生成しない",
			existing:   generated,
			persona:    &personaStub{profile: generated},
			wantCalls:  0,
			wantInText: "重視傾向",
		},
		{
			// 生成に失敗してもレビュー自体は続行させる（重視傾向が無いだけ）
			name:       "生成に失敗してもbriefは返る",
			existing:   nil,
			persona:    &personaStub{err: errors.New("ai down")},
			wantCalls:  1,
			wantInText: "企業名: 株式会社テスト",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ResumeService{}
			svc.SetCompanyRepo(&briefReaderStub{company: comp, profile: tt.existing})
			svc.SetPersonaEnsurer(tt.persona)

			brief := svc.lookupCompanyBriefFromCache("株式会社テスト")

			if tt.persona.calls != tt.wantCalls {
				t.Errorf("FetchAndSavePersona 呼び出し回数 = %d, want %d", tt.persona.calls, tt.wantCalls)
			}
			if brief == "" {
				t.Fatal("brief が空")
			}
			if !strings.Contains(brief, tt.wantInText) {
				t.Errorf("brief に %q が含まれない:\n%s", tt.wantInText, brief)
			}
		})
	}
}

// 生成器が未注入でも落ちないこと（オプション依存）
func TestLookupCompanyBrief_WithoutPersonaEnsurer(t *testing.T) {
	svc := &ResumeService{}
	svc.SetCompanyRepo(&briefReaderStub{company: &models.Company{ID: 1, Name: "株式会社テスト"}})
	if brief := svc.lookupCompanyBriefFromCache("株式会社テスト"); brief == "" {
		t.Error("brief が空")
	}
}
