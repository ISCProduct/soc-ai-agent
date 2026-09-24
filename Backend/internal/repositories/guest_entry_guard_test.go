package repositories

import (
	"strings"
	"testing"
)

// TestGuestEntryVisibilityGuard_DoesNotDependOnSubmissionRow は、審査前のゲスト投稿を
// 隠す判断が company_entry_submissions の行の有無だけに依存していないことを固定する（#1409）。
//
// 以前はその行があることを「ゲスト投稿である」唯一の識別子にしていた。行が消えると
// 「ゲスト投稿ではない」と判定され、審査前の企業が未認証の公開APIへ出た（fail-open）。
// /company-entry は誰でも無認証で投稿できるため、任意の内容を学生へ露出させられる。
//
// 実DBでの確認（投稿行を削除した審査前企業を1社用意して比較）:
//
//	旧ガード: 1件 見える（漏れる）
//	新ガード: 0件（隠れたまま）
func TestGuestEntryVisibilityGuard_DoesNotDependOnSubmissionRow(t *testing.T) {
	sql := guestEntryVisibilityGuard("r.parent_id")

	if !strings.Contains(sql, "is_guest_entry") {
		t.Error("企業行の is_guest_entry を見ていない。投稿行が消えるとガードが外れる")
	}
	// 監査用テーブル側の条件も残す。マイグレーションの埋め漏れや、
	// フラグを立てない経路が後から増えたときに片方だけで素通りさせないため。
	if !strings.Contains(sql, "company_entry_submissions") {
		t.Error("company_entry_submissions 側の条件が消えている（埋め漏れを拾えなくなる）")
	}
	if !strings.Contains(sql, "OR") {
		t.Error("2つの条件が OR で繋がっていない")
	}
	if !strings.Contains(sql, "NOT EXISTS") {
		t.Error("NOT EXISTS で除外する形が崩れている")
	}
}

// TestGuestEntryVisibilityGuard_ChecksPublishedAndActive は「公開済みかつ有効」の
// 両方を見ていることを固定する。却下されると is_active=false になるが
// data_status は published のままなので、data_status だけでは隠せない。
func TestGuestEntryVisibilityGuard_ChecksPublishedAndActive(t *testing.T) {
	sql := guestEntryVisibilityGuard("m.company_id")

	if !strings.Contains(sql, "data_status") {
		t.Error("data_status を見ていない")
	}
	if !strings.Contains(sql, "is_active") {
		t.Error("is_active を見ていない（却下された企業が相関図に残る）")
	}
}

// TestGuestEntryVisibilityGuard_AllColumns は複数端点を渡したときに
// すべてが条件へ入ることを固定する。1本でも漏れるとその経路から露出する。
func TestGuestEntryVisibilityGuard_AllColumns(t *testing.T) {
	cols := []string{"r.parent_id", "r.child_id", "r.from_id", "r.to_id"}
	sql := guestEntryVisibilityGuard(cols...)

	for _, c := range cols {
		if !strings.Contains(sql, c) {
			t.Errorf("端点 %s が条件に入っていない", c)
		}
	}
}
