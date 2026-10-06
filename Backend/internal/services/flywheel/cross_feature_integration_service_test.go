package flywheel

import (
	"encoding/json"
	"strings"
	"testing"

	"Backend/internal/models"
	"Backend/internal/repositories"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 面接レポート → user_weight_scores の反映をサービス層ごと検証する（#1554）。
//
// これまで写像の純関数だけをテストしており、サービス層のテストが1本も無かった。
// そのため次の2つが静かに壊れる余地があった。
//   - プロンプト側のルーブリックキー（interview.RubricKeys）を変えると
//     interviewScores から引けなくなり、そのカテゴリが一切書かれない
//   - evidence が空だと補正が最大 -3点側に倒れ、全カテゴリが一律で下がる
//
// ここでは INSERT の引数で保存値そのものを固定し、
// 7カテゴリ分の書き込みが揃うことを ExpectationsWereMet で確かめる。

const (
	testUserID    = uint(7)
	testSessionID = "chat-session-1"
	testOrgID     = 3
)

func newCrossFeatureService(t *testing.T) (*CrossFeatureIntegrationService, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	// カテゴリごとに SELECT → INSERT が走るので、順序ではなく引数で突き合わせる。
	mock.MatchExpectationsInOrder(false)
	return NewCrossFeatureIntegrationService(repositories.NewUserWeightScoreRepository(db)), mock
}

func newInterviewReport(scoresJSON, evidenceJSON string) *models.InterviewReport {
	return &models.InterviewReport{SessionID: 1, ScoresJSON: scoresJSON, EvidenceJSON: evidenceJSON}
}

func TestUpdateScoresFromInterviewReport_SavedValues(t *testing.T) {
	// ルーブリックは項目ごとに別の値にして、写像先の取り違えを検出できるようにする。
	rubric := map[string]int{
		"logic":         4,
		"specificity":   3,
		"ownership":     2,
		"communication": 5,
		"enthusiasm":    1,
	}
	scoresJSON, err := json.Marshal(rubric)
	require.NoError(t, err)

	fullEvidence := map[string]string{}
	for key := range rubric {
		fullEvidence[key] = strings.Repeat("あ", 200)
	}
	fullEvidenceJSON, err := json.Marshal(fullEvidence)
	require.NoError(t, err)

	tests := []struct {
		name         string
		evidenceJSON string
		stats        InterviewTranscriptStats
		// want は「保存されるべき絶対値」。evidence・発話が無いときは補正0で ×20 のまま。
		want map[string]int
	}{
		{
			name:         "evidenceも発話統計も無い（記録漏れ）",
			evidenceJSON: "",
			stats:        InterviewTranscriptStats{},
			want: map[string]int{
				"コミュニケーション力": 100, // communication=5
				"技術志向":       80,  // logic=4
				"細部志向":       60,  // specificity=3
				"リーダーシップ志向":  40,  // ownership=2
				"チャレンジ志向":    40,  // ownership=2
				"成長志向":       20,  // enthusiasm=1
				"チームワーク志向":   20,  // enthusiasm=1
			},
		},
		{
			name:         "evidence_jsonが壊れている（nil mapになる）",
			evidenceJSON: `{"logic":`,
			stats:        InterviewTranscriptStats{},
			want: map[string]int{
				"コミュニケーション力": 100,
				"技術志向":       80,
				"細部志向":       60,
				"リーダーシップ志向":  40,
				"チャレンジ志向":    40,
				"成長志向":       20,
				"チームワーク志向":   20,
			},
		},
		{
			name:         "evidenceも発話も十分（補正は上限の+5点）",
			evidenceJSON: string(fullEvidenceJSON),
			stats:        InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 120},
			want: map[string]int{
				"コミュニケーション力": 100, // 105 を 100 で clamp
				"技術志向":       85,
				"細部志向":       65,
				"リーダーシップ志向":  45,
				"チャレンジ志向":    45,
				"成長志向":       25,
				"チームワーク志向":   25,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mock := newCrossFeatureService(t)

			for category, score := range tt.want {
				// 既存行なし → SetScore（絶対値でINSERT）
				mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
					WithArgs(testUserID, testSessionID, category, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "score"}))
				mock.ExpectQuery("SELECT `organization_id` FROM `users`").
					WithArgs(testUserID).
					WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(testOrgID))
				mock.ExpectBegin()
				mock.ExpectExec("INSERT INTO `user_weight_scores`").
					WithArgs(testOrgID, testUserID, testSessionID, category, score,
						sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			}

			err := svc.UpdateScoresFromInterviewReport(testUserID, testSessionID,
				newInterviewReport(string(scoresJSON), tt.evidenceJSON), tt.stats)
			require.NoError(t, err)
			// 期待した値のINSERTが7カテゴリ分すべて起きたこと。
			// 写像キーがプロンプト側とずれると、そのカテゴリのINSERTが来ずここで落ちる。
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// TestUpdateScoresFromInterviewReport_ExistingRowBlends は既存行がある2回目以降の経路を検証する。
//
// 実ユーザーはほぼこちら（AddScore の差分加算）を通る。絶対値INSERTのテストだけでは
// movingAvgNewWeight や delta == 0 の早期リターンを壊しても気付けない。
func TestUpdateScoresFromInterviewReport_ExistingRowBlends(t *testing.T) {
	scoresJSON := `{"logic":4,"specificity":3,"ownership":2,"communication":5,"enthusiasm":1}`

	const existing = 60 // 全カテゴリの既存値
	// 新値（evidence・発話統計なし = ルーブリック×20）から
	// delta = round(既存*0.7 + 新*0.3) - 既存 を期待する。
	tests := []struct {
		category string
		newValue int
		delta    int // 0 なら書き込み自体が起きない
	}{
		{category: "コミュニケーション力", newValue: 100, delta: 12},
		{category: "技術志向", newValue: 80, delta: 6},
		{category: "細部志向", newValue: 60, delta: 0}, // 既存と同じ → UPDATE を発行しない
		{category: "リーダーシップ志向", newValue: 40, delta: -6},
		{category: "チャレンジ志向", newValue: 40, delta: -6},
		{category: "成長志向", newValue: 20, delta: -12},
		{category: "チームワーク志向", newValue: 20, delta: -12},
	}

	svc, mock := newCrossFeatureService(t)
	for i, tt := range tests {
		rowID := i + 1
		// 期待する delta を写像から導き直して、移動平均の定数のズレも検出する。
		if want := blendScore(existing, tt.newValue) - existing; want != tt.delta {
			t.Fatalf("%s: delta の期待値が写像と合っていない: %d, want %d", tt.category, tt.delta, want)
		}
		row := func() *sqlmock.Rows {
			return sqlmock.NewRows([]string{"id", "user_id", "session_id", "weight_category", "score"}).
				AddRow(rowID, testUserID, testSessionID, tt.category, existing)
		}
		// applyMovingAverage の既存値取得
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WithArgs(testUserID, testSessionID, tt.category, 1).
			WillReturnRows(row())
		if tt.delta == 0 {
			continue
		}
		// AddScore 内の再取得 → 差分UPDATE
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WithArgs(testUserID, testSessionID, tt.category, 1).
			WillReturnRows(row())
		mock.ExpectBegin()
		mock.ExpectExec("UPDATE `user_weight_scores` SET `score`=GREATEST").
			WithArgs(tt.delta, sqlmock.AnyArg(), rowID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
	}

	err := svc.UpdateScoresFromInterviewReport(testUserID, testSessionID,
		newInterviewReport(scoresJSON, ""), InterviewTranscriptStats{})
	require.NoError(t, err)
	// delta==0 の細部志向に UPDATE が来ていたら「予期しないExec」で落ちる。
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestUpdateScoresFromResumeReview_ScoreIsNilWritesNothing はスコア無しの履歴書レビューが
// user_weight_scores へ一切書き込まないことを検証する（#1529）。
//
// critical 指摘だけでペナルティを書くと「基準の無い減点」がマッチングと
// 教員向け傾向分析に静かに混ざる（docs/wiki/scoring.md §2-3）。
//
// 反映の失敗はログだけで握り潰される（他カテゴリを止めないため）ので、
// 「予期しないクエリが来たら落ちる」形では検出できない。
// そこで「スコア無しを0点と誤って扱ったときに来る SELECT」を期待として登録し、
// **その期待が満たされないこと**を成功条件にする。
func TestUpdateScoresFromResumeReview_ScoreIsNilWritesNothing(t *testing.T) {
	svc, mock := newCrossFeatureService(t)

	for _, category := range []string{"細部志向", "コミュニケーション力"} {
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WithArgs(testUserID, testSessionID, category, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "score"}))
	}

	items := []models.ResumeReviewItem{
		{Severity: "critical"}, {Severity: "critical"}, {Severity: "critical"},
	}
	err := svc.UpdateScoresFromResumeReview(testUserID, testSessionID,
		&models.ResumeReview{ID: 11, Score: nil}, items)
	require.NoError(t, err)
	require.Error(t, mock.ExpectationsWereMet(),
		"スコア無しなのに user_weight_scores を読み書きしている")
}

// TestUpdateScoresFromResumeReview_SavedValues はスコアがあるときの保存値を固定する。
// 写像は mapResumeScore（score_mapping.go）。critical 3件は 15点のペナルティ。
func TestUpdateScoresFromResumeReview_SavedValues(t *testing.T) {
	svc, mock := newCrossFeatureService(t)

	// score=85 / critical=3 → 細部志向 50+(35-15)*1.0=70、コミュニケーション力 50+(35-15)*0.6=62
	want := map[string]int{"細部志向": 70, "コミュニケーション力": 62}
	for category, score := range want {
		mock.ExpectQuery("SELECT \\* FROM `user_weight_scores`").
			WithArgs(testUserID, testSessionID, category, 1).
			WillReturnRows(sqlmock.NewRows([]string{"id", "score"}))
		mock.ExpectQuery("SELECT `organization_id` FROM `users`").
			WithArgs(testUserID).
			WillReturnRows(sqlmock.NewRows([]string{"organization_id"}).AddRow(testOrgID))
		mock.ExpectBegin()
		mock.ExpectExec("INSERT INTO `user_weight_scores`").
			WithArgs(testOrgID, testUserID, testSessionID, category, score,
				sqlmock.AnyArg(), sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
	}

	score := 85
	items := []models.ResumeReviewItem{
		{Severity: "critical"}, {Severity: "critical"}, {Severity: "critical"}, {Severity: "info"},
	}
	err := svc.UpdateScoresFromResumeReview(testUserID, testSessionID,
		&models.ResumeReview{ID: 11, Score: &score}, items)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}
