package companyportal

// 自社プロフィール編集のテスト（#1322）。
// 実行: cd Backend && go test ./internal/services/companyportal/ -run Profile -v
//
// 見たいのは「指定しなかった項目が消えないこと」と
// 「企業が変えてはいけない項目を変えられないこと」。

import (
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

type fakeCompanyRepo struct {
	company *models.Company
	updated *models.Company
}

func (r *fakeCompanyRepo) FindByID(id uint) (*models.Company, error) {
	if r.company != nil && r.company.ID == id {
		return r.company, nil
	}
	return nil, errors.New("not found")
}

func (r *fakeCompanyRepo) Update(c *models.Company) error {
	r.updated = c
	return nil
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func baseCompany() *models.Company {
	return &models.Company{
		ID: 5, Name: "テスト株式会社",
		Description: "元の概要", Industry: "IT・ソフトウェア", Location: "東京都",
		FoundedYear: 2010, EmployeeCount: 100,
		DataStatus: "draft", IsActive: true, CorporateNumber: "1234567890123",
	}
}

func TestProfileUpdate_指定した項目だけ変える(t *testing.T) {
	// 未指定とゼロ値を区別しないと、一部だけ更新したつもりで他が消える。
	repo := &fakeCompanyRepo{company: baseCompany()}
	s := NewProfileService(repo)

	got, err := s.Update(5, ProfileInput{Description: strPtr("新しい概要")})
	if err != nil {
		t.Fatalf("更新に失敗: %v", err)
	}
	if got.Description != "新しい概要" {
		t.Errorf("更新されていない: %q", got.Description)
	}
	// 指定しなかった項目が消えていないこと
	if got.Industry != "IT・ソフトウェア" || got.Location != "東京都" {
		t.Errorf("指定しない項目が変わった: industry=%q location=%q", got.Industry, got.Location)
	}
	if got.FoundedYear != 2010 || got.EmployeeCount != 100 {
		t.Errorf("数値項目が消えた: year=%d count=%d", got.FoundedYear, got.EmployeeCount)
	}
}

func TestProfileUpdate_企業が変えられない項目は据え置き(t *testing.T) {
	// 公開状態を自己申告で変えられると審査の意味が無くなる。
	// 法人番号は本人確認の根拠。ProfileInput に項目が無いことで担保する。
	repo := &fakeCompanyRepo{company: baseCompany()}
	s := NewProfileService(repo)

	got, err := s.Update(5, ProfileInput{Description: strPtr("x")})
	if err != nil {
		t.Fatalf("更新に失敗: %v", err)
	}
	if got.DataStatus != "draft" {
		t.Errorf("data_status が変わった: %s", got.DataStatus)
	}
	if !got.IsActive {
		t.Error("is_active が変わった")
	}
	if got.CorporateNumber != "1234567890123" {
		t.Errorf("法人番号が変わった: %s", got.CorporateNumber)
	}
	if got.Name != "テスト株式会社" {
		t.Errorf("企業名が変わった: %s", got.Name)
	}
}

func TestProfileUpdate_空文字で消せる(t *testing.T) {
	// 「未指定」と「空にする」は別。誤った情報を消せないと直せない。
	repo := &fakeCompanyRepo{company: baseCompany()}
	s := NewProfileService(repo)

	got, err := s.Update(5, ProfileInput{Location: strPtr("")})
	if err != nil {
		t.Fatalf("更新に失敗: %v", err)
	}
	if got.Location != "" {
		t.Errorf("空にできていない: %q", got.Location)
	}
}

func TestProfileUpdate_前後の空白を落とす(t *testing.T) {
	repo := &fakeCompanyRepo{company: baseCompany()}
	s := NewProfileService(repo)

	got, _ := s.Update(5, ProfileInput{Location: strPtr("  大阪府  ")})
	if got.Location != "大阪府" {
		t.Errorf("トリムされていない: %q", got.Location)
	}
}

func TestProfileUpdate_他社は403(t *testing.T) {
	repo := &fakeCompanyRepo{company: baseCompany()}
	s := NewProfileService(repo)

	_, err := s.Update(99, ProfileInput{Description: strPtr("x")})
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
	if repo.updated != nil {
		t.Error("他社が更新された")
	}
}

func TestProfileUpdate_companyIDが0なら403(t *testing.T) {
	s := NewProfileService(&fakeCompanyRepo{company: baseCompany()})
	_, err := s.Update(0, ProfileInput{})
	if !errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 を返すべき: %v", err)
	}
}

func TestProfileUpdate_入力の検査(t *testing.T) {
	long := make([]rune, maxProfileTextLen+1)
	for i := range long {
		long[i] = 'あ'
	}
	longShort := make([]rune, maxProfileShortLen+1)
	for i := range longShort {
		longShort[i] = 'あ'
	}

	tests := []struct {
		name    string
		in      ProfileInput
		wantErr bool
	}{
		{"通常の入力", ProfileInput{Description: strPtr("概要")}, false},
		{"概要が長すぎる", ProfileInput{Description: strPtr(string(long))}, true},
		{"業種が長すぎる", ProfileInput{Industry: strPtr(string(longShort))}, true},
		{"設立年が古すぎる", ProfileInput{FoundedYear: intPtr(1500)}, true},
		{"設立年0は未設定として通す", ProfileInput{FoundedYear: intPtr(0)}, false},
		{"従業員数が負", ProfileInput{EmployeeCount: intPtr(-1)}, true},
		{"従業員数が多すぎる", ProfileInput{EmployeeCount: intPtr(maxEmployeeCount + 1)}, true},
		{"従業員数0は通す", ProfileInput{EmployeeCount: intPtr(0)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeCompanyRepo{company: baseCompany()}
			s := NewProfileService(repo)
			_, err := s.Update(5, tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr && repo.updated != nil {
				t.Error("検証に失敗したのに保存された")
			}
		})
	}
}
