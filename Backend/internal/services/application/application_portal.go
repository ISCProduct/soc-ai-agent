package application

// 企業ポータル向けの応募者管理（#1320）。
//
// 既存の UpdateStatusAsOwner / requireCompanyOwner は company_ownerships
// （プラットフォーム上のユーザーが企業を所有している関係）を見る。
// 企業ポータルの認証は company_users で、こちらは別の主体になる。
// そのため所有権チェックはそのまま流用できず、企業スコープの検証を
// ここで行う。遷移表と値の検証（UpdateStatus）は共通のものを使う。

import (
	"fmt"

	"Backend/domain/entity"
	"Backend/internal/services/shared"
)

// PortalPendingStatuses は「未対応」とみなす選考ステータス。
//
// 終了状態(accepted / withdrawn / rejected)と未応募を除いた、
// 企業側が次の行動を取るべきもの。ダッシュボードの件数と、
// 一覧の既定の絞り込みに使う。
var PortalPendingStatuses = []string{
	"applied",
	"document_screening",
	"document_passed",
	"interview_scheduled",
	"interview_in_progress",
	"offered",
}

// ListForCompanyPortal は企業ポータルの応募者一覧を返す。
//
// companyID は JWT 由来の値のみを渡すこと。クエリパラメータから
// 受け取った値を渡すと他社の応募が見える。
func (s *ApplicationService) ListForCompanyPortal(
	companyID uint, status string, limit, offset int,
) ([]*entity.UserApplicationStatus, int64, error) {
	if companyID == 0 {
		return nil, 0, shared.ErrForbidden
	}
	if status != "" && !isValidStatus(status) {
		return nil, 0, fmt.Errorf("invalid_status: 無効なステータス %s", status)
	}
	return s.appRepo.FindByCompanyPaged(companyID, status, limit, offset)
}

// CountPendingForCompanyPortal は未対応の応募件数を返す。
func (s *ApplicationService) CountPendingForCompanyPortal(companyID uint) (int64, error) {
	if companyID == 0 {
		return 0, shared.ErrForbidden
	}
	return s.appRepo.CountByCompanyAndStatuses(companyID, PortalPendingStatuses)
}

// UpdateStatusForCompanyPortal は企業ポータルからの選考ステータス更新。
//
// 他社の応募IDを指定された場合は 403 を返す。404 にすると
// 「そのIDが存在するか」が漏れる（#1156 で採った方針）。
//
// 呼び出し元は先に owner 権限を確認すること。ここでは企業スコープだけを見る。
func (s *ApplicationService) UpdateStatusForCompanyPortal(
	applicationID, companyID uint, status string, notes *string,
) (*entity.UserApplicationStatus, error) {
	if companyID == 0 {
		return nil, shared.ErrForbidden
	}
	app, err := s.appRepo.FindByID(applicationID)
	if err != nil {
		// 存在しないIDと他社のIDを区別させない。
		return nil, shared.ErrForbidden
	}
	if app.CompanyID != companyID {
		return nil, shared.ErrForbidden
	}

	// 遷移表と値の検証は共通の実装を使う。isAdmin=true のとき
	// UpdateStatus は userID を参照しないため 0 を渡す。
	// 企業スコープの検証は上で済ませている。
	return s.UpdateStatus(applicationID, 0, status, notes, true)
}
