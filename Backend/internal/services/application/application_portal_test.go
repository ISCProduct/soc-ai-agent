package application

// 企業ポータル向け応募者管理のテスト（#1320）。
// 実行: cd Backend && go test ./internal/services/application/ -run Portal -v
//
// DBを使わずに確認できるのは「企業スコープのガード」と「未対応の定義」。
// クエリ自体の正しさは実DBが要るため、ここでは対象外。

import (
	"errors"
	"slices"
	"testing"

	"Backend/internal/services/shared"
)

func TestPortalPendingStatuses_終了状態を含まない(t *testing.T) {
	// 終了状態を「未対応」に数えると、対応済みの応募がいつまでも
	// ダッシュボードの件数に残り、企業側が消化できなくなる。
	for _, terminal := range terminalStatusList {
		if slices.Contains(PortalPendingStatuses, terminal) {
			t.Errorf("終了状態が未対応に含まれている: %s", terminal)
		}
	}
	// 未応募も企業側の対応対象ではない。
	if slices.Contains(PortalPendingStatuses, "not_applied") {
		t.Error("not_applied が未対応に含まれている")
	}
}

func TestPortalPendingStatuses_正典のステータスだけを含む(t *testing.T) {
	// 表記ゆれがあると、その値の応募が永久に件数へ乗らない。
	for _, s := range PortalPendingStatuses {
		if !isValidStatus(s) {
			t.Errorf("正典に無いステータス: %s", s)
		}
	}
}

func TestPortalPendingStatuses_進行中を取りこぼさない(t *testing.T) {
	// ValidStatuses から終了状態と not_applied を除いたものが未対応のはず。
	// 片方だけ増やすと、新しいステータスの応募が誰にも見えなくなる。
	for _, s := range ValidStatuses {
		if s == "not_applied" || slices.Contains(terminalStatusList, s) {
			continue
		}
		if !slices.Contains(PortalPendingStatuses, s) {
			t.Errorf("進行中なのに未対応に入っていない: %s", s)
		}
	}
}

// company_id が解決できないまま呼ばれたら、全社分を返さず必ず弾く。
func TestPortalScope_companyIDが0なら403(t *testing.T) {
	s := &ApplicationService{} // リポジトリ未設定。ガードを抜けたら panic する

	t.Run("一覧", func(t *testing.T) {
		_, _, err := s.ListForCompanyPortal(0, "", 10, 0)
		if !errors.Is(err, shared.ErrForbidden) {
			t.Errorf("403 を返すべき: %v", err)
		}
	})
	t.Run("未対応件数", func(t *testing.T) {
		_, err := s.CountPendingForCompanyPortal(0)
		if !errors.Is(err, shared.ErrForbidden) {
			t.Errorf("403 を返すべき: %v", err)
		}
	})
	t.Run("ステータス更新", func(t *testing.T) {
		_, err := s.UpdateStatusForCompanyPortal(1, 0, "applied", nil)
		if !errors.Is(err, shared.ErrForbidden) {
			t.Errorf("403 を返すべき: %v", err)
		}
	})
}

func TestListForCompanyPortal_無効なステータスは弾く(t *testing.T) {
	s := &ApplicationService{} // 検証で弾かれるのでリポジトリには到達しない

	_, _, err := s.ListForCompanyPortal(5, "not_a_status", 10, 0)
	if err == nil {
		t.Fatal("無効なステータスを受け付けている")
	}
	if errors.Is(err, shared.ErrForbidden) {
		t.Errorf("403 ではなく検証エラーにすべき: %v", err)
	}
}
