package schoolapproval

import (
	"errors"

	"Backend/internal/models"
)

var (
	// ErrApplicationNotFound は申請が存在しない、または対象校のものでないとき。
	ErrApplicationNotFound = errors.New("掲載申請が見つかりません")
	// ErrNotPending は承認待ち以外を再審査しようとしたとき。
	ErrNotPending = errors.New("この申請は既に処理済みです")
)

// applicationRepo は掲載申請の読み書き。
type applicationRepo interface {
	ListBySchool(schoolID uint, status string) ([]models.SchoolCompanyApplication, error)
	FindByID(id uint) (*models.SchoolCompanyApplication, error)
	UpdateStatus(id uint, status string, reviewedBy uint) error
}

// approvalWriter は承認済み企業リストへの追加。既存の SchoolRepository が満たす。
type approvalWriter interface {
	AddCompanyApproval(a *models.SchoolCompanyApproval) error
}

// ReviewService はキャリア担当による掲載申請の審査（#1507）。
type ReviewService struct {
	apps      applicationRepo
	approvals approvalWriter
}

func NewReviewService(apps applicationRepo, approvals approvalWriter) *ReviewService {
	return &ReviewService{apps: apps, approvals: approvals}
}

// List は学校の申請一覧を返す。status が空なら全件。
func (s *ReviewService) List(schoolID uint, status string) ([]models.SchoolCompanyApplication, error) {
	return s.apps.ListBySchool(schoolID, status)
}

// findScoped は申請を取得し、対象校のものか確認する。
//
// 他校の申請IDを渡されても触れないよう schoolID で照合する。存在しない場合と
// 他校の場合を同じ ErrApplicationNotFound にして、IDの存在を漏らさない。
func (s *ReviewService) findScoped(appID, schoolID uint) (*models.SchoolCompanyApplication, error) {
	app, err := s.apps.FindByID(appID)
	if err != nil {
		return nil, err
	}
	if app == nil || app.SchoolID != schoolID {
		return nil, ErrApplicationNotFound
	}
	return app, nil
}

// Approve は申請を承認し、承認済み企業リストに追加する。
//
// 承認済みリストへの追加を先に行う。ここが失敗したのに status だけ approved に
// なると、学生に企業が出ないのに承認済み表示になり不整合が起きる。既に承認済み
// （一意制約違反）はべき等に扱う。
func (s *ReviewService) Approve(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error) {
	app, err := s.findScoped(appID, schoolID)
	if err != nil {
		return nil, err
	}
	if !app.IsPending() {
		return nil, ErrNotPending
	}
	if err := s.approvals.AddCompanyApproval(&models.SchoolCompanyApproval{SchoolID: schoolID, CompanyID: app.CompanyID}); err != nil {
		if !isDuplicateEntryErr(err) {
			return nil, err
		}
	}
	if err := s.apps.UpdateStatus(appID, models.SchoolCompanyApplicationApproved, reviewerID); err != nil {
		return nil, err
	}
	app.Status = models.SchoolCompanyApplicationApproved
	app.ReviewedBy = &reviewerID
	return app, nil
}

// Reject は申請を却下する。承認済みリストには触れない。
func (s *ReviewService) Reject(appID, schoolID, reviewerID uint) (*models.SchoolCompanyApplication, error) {
	app, err := s.findScoped(appID, schoolID)
	if err != nil {
		return nil, err
	}
	if !app.IsPending() {
		return nil, ErrNotPending
	}
	if err := s.apps.UpdateStatus(appID, models.SchoolCompanyApplicationRejected, reviewerID); err != nil {
		return nil, err
	}
	app.Status = models.SchoolCompanyApplicationRejected
	app.ReviewedBy = &reviewerID
	return app, nil
}
