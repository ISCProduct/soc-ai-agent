package companyportal

// 企業ポータルの自社プロフィール編集（#1322）。
//
// 企業ポータルは自社の情報を読むことしかできず、間違いの修正も
// 運営への依頼になっていた。導入社数が増えたときに真っ先に壊れる。

import (
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

type companyRepository interface {
	FindByID(id uint) (*models.Company, error)
	Update(company *models.Company) error
}

type ProfileService struct {
	repo companyRepository
}

func NewProfileService(repo companyRepository) *ProfileService {
	return &ProfileService{repo: repo}
}

// ProfileInput は企業が自分で編集できる項目。
//
// ここに無い項目は企業からは変えられない。特に data_status / is_active /
// corporate_number は意図的に外している。公開状態を自己申告で変えられると
// 審査の意味が無くなり、法人番号は本人確認の根拠になっているため。
//
// ポインタ型は「指定されなかった項目は変更しない」を表す。
// 未指定とゼロ値を区別しないと、一部だけ更新したつもりで他が消える。
type ProfileInput struct {
	Description    *string
	Industry       *string
	Location       *string
	WebsiteURL     *string
	LogoURL        *string
	FoundedYear    *int
	EmployeeCount  *int
	Culture        *string
	WorkStyle      *string
	WelfareDetails *string
	MainBusiness   *string
}

const (
	maxProfileTextLen  = 5000
	maxProfileShortLen = 255
	minFoundedYear     = 1800
	maxEmployeeCount   = 3_000_000
)

func (in ProfileInput) validate() error {
	if in.Description != nil && len([]rune(*in.Description)) > maxProfileTextLen {
		return &shared.ValidationError{Message: "企業概要が長すぎます"}
	}
	for _, f := range []struct {
		v    *string
		name string
	}{
		{in.Industry, "業種"},
		{in.Location, "所在地"},
		{in.WebsiteURL, "URL"},
		{in.LogoURL, "ロゴURL"},
	} {
		if f.v != nil && len([]rune(*f.v)) > maxProfileShortLen {
			return &shared.ValidationError{Message: f.name + "が長すぎます"}
		}
	}
	// 年と人数は明らかな誤りだけ弾く。厳しくすると正しい値まで入らなくなる。
	if in.FoundedYear != nil && *in.FoundedYear != 0 && *in.FoundedYear < minFoundedYear {
		return &shared.ValidationError{Message: "設立年が不正です"}
	}
	if in.EmployeeCount != nil && (*in.EmployeeCount < 0 || *in.EmployeeCount > maxEmployeeCount) {
		return &shared.ValidationError{Message: "従業員数が不正です"}
	}
	return nil
}

// Get は自社のプロフィールを返す。
func (s *ProfileService) Get(companyID uint) (*models.Company, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	company, err := s.repo.FindByID(companyID)
	if err != nil || company == nil {
		return nil, shared.ErrForbidden
	}
	return company, nil
}

// Update は自社のプロフィールを更新する。
//
// companyID は JWT 由来の値のみを渡すこと。他社のIDを渡せば他社を書き換えられる。
func (s *ProfileService) Update(companyID uint, in ProfileInput) (*models.Company, error) {
	company, err := s.Get(companyID)
	if err != nil {
		return nil, err
	}
	if err := in.validate(); err != nil {
		return nil, err
	}

	applyString(&company.Description, in.Description)
	applyString(&company.Industry, in.Industry)
	applyString(&company.Location, in.Location)
	applyString(&company.WebsiteURL, in.WebsiteURL)
	applyString(&company.LogoURL, in.LogoURL)
	applyString(&company.Culture, in.Culture)
	applyString(&company.WorkStyle, in.WorkStyle)
	applyString(&company.WelfareDetails, in.WelfareDetails)
	applyString(&company.MainBusiness, in.MainBusiness)
	if in.FoundedYear != nil {
		company.FoundedYear = *in.FoundedYear
	}
	if in.EmployeeCount != nil {
		company.EmployeeCount = *in.EmployeeCount
	}

	if err := s.repo.Update(company); err != nil {
		return nil, err
	}
	return company, nil
}

func applyString(dst *string, src *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}
