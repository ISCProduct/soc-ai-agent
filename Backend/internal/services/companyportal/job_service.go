// Package companyportal は企業ポータル固有のユースケースを持つ。
//
// 企業ポータルの認証主体は company_users で、プラットフォーム側の
// company_ownerships とは別。そのため既存のオーナー向けサービスは
// そのまま流用できず、企業スコープの検証をここで行う。
package companyportal

import (
	"errors"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared"
)

// 求人の公開状態。company_job_positions.data_status の値。
const (
	JobStatusDraft     = "draft"
	JobStatusPublished = "published"
)

// maxJobsPerCompany は1社あたりの求人数の上限。
// 上限が無いと、誤操作や自動投稿で学生側の企業詳細が埋まる。
const maxJobsPerCompany = 200

// ErrJobLimitReached は求人数の上限に達したことを表す。
var ErrJobLimitReached = errors.New("求人数の上限に達しました")

// JobVisibility は求人が学生に見えるかどうかと、その理由を表す。
//
// 求人を published にしても、企業本体が未公開なら学生側の企業詳細には
// 出ないことがある。「公開したつもりで見えていない」状態を UI で
// 黙らせないために、企業側の状態も一緒に返す(#1321)。
//
// 公開自体は拒否しない。現状 842社すべてが draft / provisional で
// (#1077 / #1078)、拒否すると求人機能がまったく使えなくなる。
// 企業の公開条件の整理はそちらのIssueの範囲。
type JobVisibility struct {
	Job              *models.CompanyJobPosition
	CompanyPublished bool
}

type jobRepository interface {
	FindByID(id uint) (*models.Company, error)
	FindJobPositionByID(id uint) (*models.CompanyJobPosition, error)
	ListJobPositions(companyID, schoolID *uint, limit int) ([]models.CompanyJobPosition, error)
	CreateJobPosition(position *models.CompanyJobPosition) error
	UpdateJobPosition(position *models.CompanyJobPosition) error
}

type JobService struct {
	repo jobRepository
}

func NewJobService(repo jobRepository) *JobService {
	return &JobService{repo: repo}
}

// JobInput は求人の作成・更新で受け取る値。
//
// company_id は含めない。JWT 由来の値だけを使い、
// ボディで指定された企業は無視する(#1319 の共通方針)。
type JobInput struct {
	Title           string
	Description     string
	JobURL          string
	JobCategoryID   uint
	MinSalary       int
	MaxSalary       int
	EmploymentType  string
	WorkLocation    string
	RemoteOption    bool
	RequiredSkills  string
	PreferredSkills string
}

// Validate は入力の最小限の検査を行う。
//
// 必須は title だけにする。入力負担を上げると求人が0件のままになる(#1321)。
func (in JobInput) Validate() error {
	if strings.TrimSpace(in.Title) == "" {
		return &shared.ValidationError{Message: "職種名は必須です"}
	}
	if len([]rune(in.Title)) > 255 {
		return &shared.ValidationError{Message: "職種名が長すぎます"}
	}
	if in.MinSalary < 0 || in.MaxSalary < 0 {
		return &shared.ValidationError{Message: "年収に負の値は指定できません"}
	}
	if in.MinSalary > 0 && in.MaxSalary > 0 && in.MinSalary > in.MaxSalary {
		return &shared.ValidationError{Message: "最低年収が最高年収を上回っています"}
	}
	return nil
}

// List は自社の求人を下書きも含めて返す。
//
// 学生向けの FindJobPositionsByCompany は公開済みしか返さないため、
// 企業ポータルでは使えない。下書きが見えないと編集できない。
func (s *JobService) List(companyID uint) ([]models.CompanyJobPosition, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	return s.repo.ListJobPositions(&companyID, nil, maxJobsPerCompany)
}

// Create は求人を下書きとして作る。公開は別操作(Publish)にする。
func (s *JobService) Create(companyID uint, in JobInput) (*models.CompanyJobPosition, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	existing, err := s.repo.ListJobPositions(&companyID, nil, maxJobsPerCompany+1)
	if err != nil {
		return nil, err
	}
	if len(existing) >= maxJobsPerCompany {
		return nil, ErrJobLimitReached
	}

	job := &models.CompanyJobPosition{CompanyID: companyID, IsActive: true, DataStatus: JobStatusDraft}
	applyJobInput(job, in)
	if err := s.repo.CreateJobPosition(job); err != nil {
		return nil, err
	}
	return job, nil
}

// Update は求人の内容を更新する。公開状態はここでは変えない。
func (s *JobService) Update(jobID, companyID uint, in JobInput) (*models.CompanyJobPosition, error) {
	job, err := s.ownedJob(jobID, companyID)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	applyJobInput(job, in)
	if err := s.repo.UpdateJobPosition(job); err != nil {
		return nil, err
	}
	return job, nil
}

// SetPublished は求人の公開・非公開を切り替える。
//
// 削除は提供しない。応募が紐づくため、非公開化で対応する(#1321)。
//
// 公開できるのは企業本体が published のときだけ。求人だけ公開しても
// 学生側の企業詳細に出ないので、成功したように見せてはいけない。
func (s *JobService) SetPublished(jobID, companyID uint, published bool) (*JobVisibility, error) {
	job, err := s.ownedJob(jobID, companyID)
	if err != nil {
		return nil, err
	}

	if published {
		job.DataStatus = JobStatusPublished
		job.IsActive = true
	} else {
		// 非公開は data_status を戻す。is_active は残し、
		// 「取り下げ」と「掲載終了」を将来分けられるようにしておく。
		job.DataStatus = JobStatusDraft
	}

	if err := s.repo.UpdateJobPosition(job); err != nil {
		return nil, err
	}

	return &JobVisibility{Job: job, CompanyPublished: s.companyPublished(companyID)}, nil
}

// companyPublished は企業本体が学生に見える状態かを返す。
// 取得に失敗した場合は false にする。見えると誤って伝えるより安全側に倒す。
func (s *JobService) companyPublished(companyID uint) bool {
	company, err := s.repo.FindByID(companyID)
	if err != nil || company == nil {
		return false
	}
	return company.DataStatus == "published" && company.IsActive
}

// CompanyPublished は企業本体の公開状態を返す。一覧表示の注意書きに使う。
func (s *JobService) CompanyPublished(companyID uint) bool {
	if companyID == 0 {
		return false
	}
	return s.companyPublished(companyID)
}

// ownedJob は自社の求人だけを返す。
//
// 他社の求人IDと存在しないIDを区別せず 403 にする。
// 404 と分けると、そのIDが存在するかを総当たりで調べられる(#1156)。
func (s *JobService) ownedJob(jobID, companyID uint) (*models.CompanyJobPosition, error) {
	if companyID == 0 || jobID == 0 {
		return nil, shared.ErrForbidden
	}
	job, err := s.repo.FindJobPositionByID(jobID)
	if err != nil || job == nil {
		return nil, shared.ErrForbidden
	}
	if job.CompanyID != companyID {
		return nil, shared.ErrForbidden
	}
	return job, nil
}

func applyJobInput(job *models.CompanyJobPosition, in JobInput) {
	job.Title = strings.TrimSpace(in.Title)
	job.Description = in.Description
	job.JobURL = strings.TrimSpace(in.JobURL)
	job.JobCategoryID = in.JobCategoryID
	job.MinSalary = in.MinSalary
	job.MaxSalary = in.MaxSalary
	job.EmploymentType = strings.TrimSpace(in.EmploymentType)
	job.WorkLocation = strings.TrimSpace(in.WorkLocation)
	job.RemoteOption = in.RemoteOption
	job.RequiredSkills = in.RequiredSkills
	job.PreferredSkills = in.PreferredSkills
}
