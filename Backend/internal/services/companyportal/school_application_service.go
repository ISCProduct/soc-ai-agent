package companyportal

import (
	"errors"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// ErrDuplicateApplication は同一 school×company で承認待ちが既にあるとき。
var ErrDuplicateApplication = errors.New("この学校へは既に申請済みです")

// schoolAppRepo は掲載申請の永続化（#1506）。
type schoolAppRepo interface {
	ListByCompany(companyID uint) ([]models.SchoolCompanyApplication, error)
	HasPending(schoolID, companyID uint) (bool, error)
	Create(app *models.SchoolCompanyApplication) error
	FindByID(id uint) (*models.SchoolCompanyApplication, error)
	Delete(id uint) error
}

// SchoolApplicationService は企業→学校の掲載申請。
type SchoolApplicationService struct {
	repo schoolAppRepo
}

func NewSchoolApplicationService(repo schoolAppRepo) *SchoolApplicationService {
	return &SchoolApplicationService{repo: repo}
}

// List は企業の申請一覧を返す。
func (s *SchoolApplicationService) List(companyID uint) ([]models.SchoolCompanyApplication, error) {
	return s.repo.ListByCompany(companyID)
}

// Apply は掲載申請を作る。
//
// 承認待ちが既にあれば重複を作らない。ここで弾かないと、キャリア担当の
// 承認キューに同じ申請が並ぶ。
func (s *SchoolApplicationService) Apply(companyID, schoolID, appliedBy uint) (*models.SchoolCompanyApplication, error) {
	if schoolID == 0 {
		return nil, &shared.ValidationError{Message: "学校を指定してください"}
	}
	dup, err := s.repo.HasPending(schoolID, companyID)
	if err != nil {
		return nil, err
	}
	if dup {
		return nil, ErrDuplicateApplication
	}
	app := &models.SchoolCompanyApplication{
		SchoolID:  schoolID,
		CompanyID: companyID,
		Status:    models.SchoolCompanyApplicationPending,
		AppliedBy: appliedBy,
	}
	if err := s.repo.Create(app); err != nil {
		return nil, err
	}
	return app, nil
}

// Cancel は自社の申請を取り消す。
//
// 他社の申請IDを指定されても消せないよう、company_id で所有を確認する。
// 存在の漏洩を防ぐため、他社のものも「権限なし」で返す（#1156 の方針）。
func (s *SchoolApplicationService) Cancel(id, companyID uint) error {
	app, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if app == nil || app.CompanyID != companyID {
		return shared.ErrForbidden
	}
	return s.repo.Delete(id)
}
