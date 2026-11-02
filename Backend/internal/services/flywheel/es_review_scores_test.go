package flywheel

import (
	"testing"

	"Backend/internal/models"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func newESReview() *models.ESReview {
	return &models.ESReview{
		ID: 11, UserID: testUserID,
		SpecificityScore: 10, StarScore: 10, LengthBalanceScore: 10,
	}
}

// 診断セッションがまだ無い利用者には何も書かない（#1531）。
//
// FindLatestByUser は「最新1行のセッション」を選んでそのセッションのスコアを全件返す。
// ES専用のセッションを作ると「最新セッション＝ESの2カテゴリだけ」になり、
// 残り8カテゴリが欠落して中立50で埋められる。マッチングが静かに歪むので、
// 書き込み先が無いときは書かないことをここで固定する。
func TestUpdateScoresFromESReview_NoDiagnosisSessionWritesNothing(t *testing.T) {
	svc, mock := newCrossFeatureService(t)

	// 診断セッションの引き当てが 0 件。
	mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}))

	require.NoError(t, svc.UpdateScoresFromESReview(testUserID, newESReview()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// 反映先の決定そのものを固定する。
//
// 反映側は書き込みエラーをログに流すので、「書かれていないこと」を sqlmock の
// 期待だけで確かめても検出力が出ない（想定外のクエリがエラーになっても握られる）。
// 専用セッションを作らないという判断を、戻り値で直接確かめる。
func TestESTargetSession(t *testing.T) {
	t.Run("診断セッションが無ければ ok=false", func(t *testing.T) {
		svc, mock := newCrossFeatureService(t)
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}))

		sessionID, ok, err := svc.esTargetSession(testUserID)
		require.NoError(t, err)
		require.False(t, ok, "専用セッションを作ってはいけない")
		require.Empty(t, sessionID)
	})

	t.Run("既存の診断セッションを返す", func(t *testing.T) {
		svc, mock := newCrossFeatureService(t)
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}).AddRow(1, testSessionID))

		sessionID, ok, err := svc.esTargetSession(testUserID)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, testSessionID, sessionID, "既存セッションへ合流すること")
	})
}

// 既存の診断セッションへ合流させる。専用セッションを作らない。
func TestUpdateScoresFromESReview_WritesIntoLatestDiagnosisSession(t *testing.T) {
	svc, mock := newCrossFeatureService(t)

	mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "session_id"}).AddRow(1, testSessionID))

	// 反映先は2カテゴリだけ。どちらも testSessionID に対して読み書きする。
	for _, category := range []string{"コミュニケーション力", "細部志向"} {
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WithArgs(testUserID, testSessionID, category, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "score"}))
		mock.ExpectQuery("SELECT `organization_id` FROM `users`").
			WithArgs(testUserID).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(testOrgID))
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO `user_weight_scores`").
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	require.NoError(t, svc.UpdateScoresFromESReview(testUserID, newESReview()))
	require.NoError(t, mock.ExpectationsWereMet())
}

// nil は何もしない（呼び出し側が保存に失敗した場合など）。
func TestUpdateScoresFromESReview_NilReviewIsNoop(t *testing.T) {
	svc, mock := newCrossFeatureService(t)
	require.NoError(t, svc.UpdateScoresFromESReview(testUserID, nil))
	require.NoError(t, mock.ExpectationsWereMet())
}
