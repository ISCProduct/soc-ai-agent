package repository

import (
	"Backend/internal/models"
	"time"
)

// CompanyEntryDraft は、掲載申請(ゲスト投稿)で一緒に作る行。
// 子の行の CompanyID は、企業を作ったあとに Repository が入れる。
type CompanyEntryDraft struct {
	Company       *models.Company
	Submission    *models.CompanyEntrySubmission
	JobPositions  []*models.CompanyJobPosition
	WeightProfile *models.CompanyWeightProfile // nil なら作らない
	Graduates     []*models.GraduateEmployment
}

// CompanyEntryRepository は掲載申請と、申請に紐づく招待メール・所有権の DB 操作を担う。
type CompanyEntryRepository interface {
	// CreateDraft は企業・求人・重みプロファイル・卒業生・申請を、1トランザクションで作る。
	// 保存の順序は 企業 → 求人 → 重みプロファイル → 卒業生 → 申請。どれかが失敗したら全体を巻き戻す。
	CreateDraft(draft CompanyEntryDraft) error

	// FindSubmission は申請を1件読む。無ければ ErrNotFound。
	FindSubmission(submissionID uint) (*models.CompanyEntrySubmission, error)
	// FindCompanyName は企業の id と name だけを読む。
	FindCompanyName(companyID uint) (*models.Company, error)

	// MarkInviteEmailSent は招待メールの送信済みを記録する(試行回数を1増やし、直前のエラーを消す)。
	MarkInviteEmailSent(submissionID uint, at time.Time) error
	// MarkInviteEmailFailed は招待メールの送信失敗を記録する(試行回数を1増やす)。
	MarkInviteEmailFailed(submissionID uint, msg string) error

	// ClaimOwnership は企業の所有者を、まだ居なければ userID にする。
	// 企業には所有者が1人しか居られない(company_id の一意制約)ので、既に別のユーザーが
	// 所有していたときは、エラーにならず、その既存の行(別のユーザーの UserID)が返る。
	ClaimOwnership(companyID, userID uint) (*models.CompanyOwnership, error)
	// MarkSubmissionClaimed は申請を claimed にする。
	MarkSubmissionClaimed(submissionID uint) error
}
