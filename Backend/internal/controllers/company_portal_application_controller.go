package controllers

// 企業ポータルのダッシュボードと応募者管理（#1320）。
//
// company_id は必ず JWT 由来の値を使い、クエリパラメータやボディから
// 受け取った企業指定は無視する（#1094 / #1319 の共通方針）。
// 他社リソースを指定された場合は 404 ではなく 403 を返す。
// 404 にすると「そのIDが存在するか」が漏れる（#1156）。

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"Backend/domain/entity"
	"Backend/internal/middleware"
	"Backend/internal/services/shared"

	"github.com/labstack/echo/v4"
)

// newCandidateWindowDays は「新着候補者」とみなす日数。
// ダッシュボードで今週の動きが分かる粒度にする。
const newCandidateWindowDays = 7

type portalApplicationLister interface {
	ListForCompanyPortal(companyID uint, status string, limit, offset int) ([]*entity.UserApplicationStatus, int64, error)
	CountPendingForCompanyPortal(companyID uint) (int64, error)
	UpdateStatusForCompanyPortal(applicationID, companyID uint, status string, notes *string) (*entity.UserApplicationStatus, error)
}

type portalJobCounter interface {
	CountPublishedJobPositions(companyID uint) (int64, error)
}

type portalStudentCounter interface {
	CountNewStudents(companyID uint, since time.Time) (int64, error)
	// VisibleStudentNames はスカウト公開に同意した学生の氏名のみを返す。
	// 同意していない応募者は含まれない。
	VisibleStudentNames(companyID uint, userIDs []uint) (map[uint]string, error)
}

type CompanyPortalApplicationController struct {
	apps     portalApplicationLister
	jobs     portalJobCounter
	students portalStudentCounter
}

func NewCompanyPortalApplicationController(
	apps portalApplicationLister,
	jobs portalJobCounter,
	students portalStudentCounter,
) *CompanyPortalApplicationController {
	return &CompanyPortalApplicationController{apps: apps, jobs: jobs, students: students}
}

// DashboardResponse はポータルトップの集計。
//
// 応募も求人も0件の状態が通常なので、その場合もエラーにせず0を返す。
// 「何もない」ことが分かれば、UI 側が次の導線を出せる。
type DashboardResponse struct {
	PendingApplications int64 `json:"pending_applications"`
	PublishedJobs       int64 `json:"published_jobs"`
	NewCandidates       int64 `json:"new_candidates"`
	// NewCandidateWindowDays は new_candidates が何日ぶんかを示す。
	// UI 側で「過去7日」と出すために返す。
	NewCandidateWindowDays int `json:"new_candidate_window_days"`
}

// Dashboard GET /api/company-portal/dashboard
//
// 集計を1リクエストにまとめる。画面表示のたびに3本叩かせると、
// 遅いうえに件数の基準時刻がずれる。
func (c *CompanyPortalApplicationController) Dashboard(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}

	pending, err := c.apps.CountPendingForCompanyPortal(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	jobs, err := c.jobs.CountPublishedJobPositions(companyID)
	if err != nil {
		return echoInternalError(err)
	}
	since := time.Now().AddDate(0, 0, -newCandidateWindowDays)
	candidates, err := c.students.CountNewStudents(companyID, since)
	if err != nil {
		return echoInternalError(err)
	}

	return ctx.JSON(http.StatusOK, DashboardResponse{
		PendingApplications:    pending,
		PublishedJobs:          jobs,
		NewCandidates:          candidates,
		NewCandidateWindowDays: newCandidateWindowDays,
	})
}

// applicationItem は応募1件分。
//
// entity をそのまま返すとJSONタグが無くGoのフィールド名で出るため、
// 他のAPIと同じ snake_case になるよう明示的に組み立てる
// （管理者向けの jsonAdminApplicationList と同じ方針）。
//
// StudentName はスカウト公開に同意した学生のみ入る。同意が無ければ空文字。
// 「自社に応募した学生を同意の有無に関わらず見せるか」は #1319 で未決定のため、
// 決まるまでは既存の同意ルール(visibleStudents)をそのまま適用する。
type applicationItem struct {
	ID              uint       `json:"id"`
	UserID          uint       `json:"user_id"`
	StudentName     string     `json:"student_name"`
	Status          string     `json:"status"`
	Notes           string     `json:"notes"`
	AppliedAt       *time.Time `json:"applied_at"`
	StatusUpdatedAt *time.Time `json:"status_updated_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toApplicationItem(a *entity.UserApplicationStatus, studentName string) applicationItem {
	return applicationItem{
		ID:              a.ID,
		UserID:          a.UserID,
		StudentName:     studentName,
		Status:          a.Status,
		Notes:           a.Notes,
		AppliedAt:       a.AppliedAt,
		StatusUpdatedAt: a.StatusUpdatedAt,
		CreatedAt:       a.CreatedAt,
	}
}

type applicationListResponse struct {
	Applications []applicationItem `json:"applications"`
	Total        int64             `json:"total"`
	Limit        int               `json:"limit"`
	Offset       int               `json:"offset"`
}

// List GET /api/company-portal/applications
//
// 絞り込みは status のみ。求人での絞り込み(job_id)は
// user_application_statuses に求人の紐付けが無いため提供しない(#1321 の範囲)。
func (c *CompanyPortalApplicationController) List(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}

	limit := echoIntQuery(ctx, "limit", 30)
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	offset := max(echoIntQuery(ctx, "offset", 0), 0)

	status := strings.TrimSpace(ctx.QueryParam("status"))
	apps, total, err := c.apps.ListForCompanyPortal(companyID, status, limit, offset)
	if err != nil {
		return mapPortalApplicationError(err)
	}

	// 氏名は同意済みの学生だけ付ける。取れなかった応募者は空文字のままにし、
	// UI 側で「非公開」と出す。ここで users を直接引くと同意チェックを
	// 迂回する経路が増えるため、必ず visibleStudents 経由で解決する。
	userIDs := make([]uint, 0, len(apps))
	for _, a := range apps {
		userIDs = append(userIDs, a.UserID)
	}
	names, err := c.students.VisibleStudentNames(companyID, userIDs)
	if err != nil {
		return echoInternalError(err)
	}

	items := make([]applicationItem, len(apps))
	for i, a := range apps {
		items[i] = toApplicationItem(a, names[a.UserID])
	}

	return ctx.JSON(http.StatusOK, applicationListResponse{
		Applications: items,
		Total:        total,
		Limit:        limit,
		Offset:       offset,
	})
}

type updateApplicationStatusBody struct {
	Status string  `json:"status"`
	Notes  *string `json:"notes"`
}

// UpdateStatus PATCH /api/company-portal/applications/:id/status
//
// 選考ステータスの変更は破壊的操作なので owner のみ許す（#1319 の共通方針）。
func (c *CompanyPortalApplicationController) UpdateStatus(ctx echo.Context) error {
	companyID, ok := echoCompanyID(ctx)
	if !ok {
		return newAPIError(http.StatusUnauthorized, ErrCodeUnauthorized, "Unauthorized")
	}
	if !middleware.CompanyUserIsOwner(ctx.Request().Context()) {
		return newAPIError(http.StatusForbidden, ErrCodeForbidden, "選考ステータスの更新は管理者のみ行えます")
	}

	id, apiErr := echoUintParam(ctx, "id")
	if apiErr != nil {
		return apiErr
	}

	var body updateApplicationStatusBody
	if err := ctx.Bind(&body); err != nil {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "Invalid request body")
	}
	if strings.TrimSpace(body.Status) == "" {
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, "status は必須です")
	}

	app, err := c.apps.UpdateStatusForCompanyPortal(id, companyID, body.Status, body.Notes)
	if err != nil {
		return mapPortalApplicationError(err)
	}
	// 一覧と同じ形で返す。氏名は一覧で解決済みのものを使うため、ここでは空。
	return ctx.JSON(http.StatusOK, toApplicationItem(app, ""))
}

// mapPortalApplicationError はサービス層のエラーをHTTPへ写す。
//
// 他社リソースと存在しないIDはどちらも 403 にまとめる。
// 区別すると、存在するIDかどうかを総当たりで調べられる。
func mapPortalApplicationError(err error) error {
	if errors.Is(err, shared.ErrForbidden) {
		return newAPIError(http.StatusForbidden, ErrCodeForbidden, "この応募を操作する権限がありません")
	}

	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "invalid_status_transition:"):
		return newAPIError(http.StatusConflict, ErrCodeInvalidStatus, msg)
	case strings.HasPrefix(msg, "application_already_closed:"):
		return newAPIError(http.StatusConflict, ErrCodeInvalidStatus, msg)
	case strings.HasPrefix(msg, "invalid_status:"):
		return newAPIError(http.StatusBadRequest, ErrCodeValidationError, msg)
	case strings.HasPrefix(msg, "application_not_found:"):
		// 企業スコープ外と区別しない。
		return newAPIError(http.StatusForbidden, ErrCodeForbidden, "この応募を操作する権限がありません")
	}
	return echoInternalError(err)
}
