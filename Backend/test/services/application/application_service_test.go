package application_test

// ApplicationServiceの状態遷移エンジンテスト
//
// 実行: cd Backend && go test ./test/services/... -run Application -v

import (
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/application"

	"github.com/stretchr/testify/assert"
)

// ---- ValidStatuses ----

func TestApplicationService_ValidStatuses_ContainsExpected(t *testing.T) {
	expected := []string{
		"not_applied",
		"applied",
		"document_screening",
		"document_passed",
		"interview_scheduled",
		"interview_in_progress",
		"offered",
		"accepted",
		"withdrawn",
		"rejected",
	}
	for _, s := range expected {
		assert.Contains(t, application.ValidStatuses, s, "ValidStatuses に %s が含まれていない", s)
	}
}

func TestApplicationService_ValidStatuses_DoesNotContainLegacy(t *testing.T) {
	legacy := []string{"interview", "declined"}
	for _, s := range legacy {
		assert.NotContains(t, application.ValidStatuses, s, "ValidStatuses に廃止ステータス %s が含まれている", s)
	}
}

func TestFlywheelPassedStatuses_AlignedWithValidStatuses(t *testing.T) {
	notPassed := map[string]bool{
		"not_applied":        true,
		"applied":            true,
		"document_screening": true,
		"withdrawn":          true,
		"rejected":           true,
	}
	for _, s := range application.ValidStatuses {
		if notPassed[s] {
			assert.NotContains(t, models.FlywheelPassedStatuses, s, "非通過 %s が通過定数に入っている", s)
			continue
		}
		assert.Contains(t, models.FlywheelPassedStatuses, s, "ValidStatuses の %s が通過定数に無い", s)
	}
	for _, s := range models.FlywheelPassedStatuses {
		assert.Contains(t, application.ValidStatuses, s, "通過定数 %s が ValidStatuses に無い", s)
	}
}

// ---- canTransition (exported via CanTransition for testing) ----
// サービス内部の canTransition を間接検証するため UpdateStatus のロジックを
// テーブル駆動テストで網羅する。

type transitionCase struct {
	name    string
	current string
	next    string
	isAdmin bool
	allowed bool
}

func TestCanTransition_UserTransitions(t *testing.T) {
	cases := []transitionCase{
		// 許可: ユーザーによる辞退
		{"applied→withdrawn", "applied", "withdrawn", false, true},
		{"document_screening→withdrawn", "document_screening", "withdrawn", false, true},
		{"document_passed→withdrawn", "document_passed", "withdrawn", false, true},
		{"interview_scheduled→withdrawn", "interview_scheduled", "withdrawn", false, true},
		{"interview_in_progress→withdrawn", "interview_in_progress", "withdrawn", false, true},
		{"offered→withdrawn", "offered", "withdrawn", false, true},
		// 許可: ユーザーによる内定承諾
		{"offered→accepted", "offered", "accepted", false, true},
		// 禁止: ユーザーは選考進捗を進められない
		{"applied→accepted (user)", "applied", "accepted", false, false},
		{"applied→document_screening (user)", "applied", "document_screening", false, false},
		{"document_screening→document_passed (user)", "document_screening", "document_passed", false, false},
		{"document_passed→interview_scheduled (user)", "document_passed", "interview_scheduled", false, false},
		{"interview_scheduled→interview_in_progress (user)", "interview_scheduled", "interview_in_progress", false, false},
		{"interview_in_progress→offered (user)", "interview_in_progress", "offered", false, false},
		{"interview_in_progress→rejected (user)", "interview_in_progress", "rejected", false, false},
		// 禁止: 後戻り
		{"document_passed→applied (user)", "document_passed", "applied", false, false},
		{"offered→applied (user)", "offered", "applied", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := application.CanTransition(tc.current, tc.next, tc.isAdmin)
			assert.Equal(t, tc.allowed, got)
		})
	}
}

func TestCanTransition_AdminTransitions(t *testing.T) {
	cases := []transitionCase{
		// 許可: 管理者による選考進捗
		{"not_applied→applied (admin)", "not_applied", "applied", true, true},
		{"applied→document_screening (admin)", "applied", "document_screening", true, true},
		{"document_screening→document_passed (admin)", "document_screening", "document_passed", true, true},
		{"document_screening→rejected (admin)", "document_screening", "rejected", true, true},
		{"document_passed→interview_scheduled (admin)", "document_passed", "interview_scheduled", true, true},
		{"interview_scheduled→interview_in_progress (admin)", "interview_scheduled", "interview_in_progress", true, true},
		{"interview_in_progress→offered (admin)", "interview_in_progress", "offered", true, true},
		{"interview_in_progress→rejected (admin)", "interview_in_progress", "rejected", true, true},
		// 許可: 管理者による辞退・承諾
		{"applied→withdrawn (admin)", "applied", "withdrawn", true, true},
		{"offered→accepted (admin)", "offered", "accepted", true, true},
		// 禁止: offered→rejected は直接遷移禁止（要件定義 §7.2 R-4）
		{"offered→rejected (admin)", "offered", "rejected", true, false},
		// 禁止: 後戻り
		{"document_screening→applied (admin)", "document_screening", "applied", true, false},
		{"offered→applied (admin)", "offered", "applied", true, false},
		// 禁止: 終了状態からの遷移（通常更新）
		{"accepted→applied (admin)", "accepted", "applied", true, false},
		{"withdrawn→applied (admin)", "withdrawn", "applied", true, false},
		{"rejected→applied (admin)", "rejected", "applied", true, false},
		{"interview→offered (admin, legacy)", "interview", "offered", true, true},
		{"declined→applied (admin, legacy terminal)", "declined", "applied", true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := application.CanTransition(tc.current, tc.next, tc.isAdmin)
			assert.Equal(t, tc.allowed, got)
		})
	}
}

func TestCanTransition_TerminalStatuses(t *testing.T) {
	// 終了状態からはユーザー・管理者とも通常遷移不可
	terminals := []string{"accepted", "withdrawn", "rejected"}
	nexts := []string{"applied", "document_screening", "offered", "accepted"}
	for _, terminal := range terminals {
		for _, next := range nexts {
			t.Run(terminal+"→"+next, func(t *testing.T) {
				assert.False(t, application.CanTransition(terminal, next, false))
				assert.False(t, application.CanTransition(terminal, next, true))
			})
		}
	}
}
