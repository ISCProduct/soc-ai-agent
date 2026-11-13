package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/domain/valueobject"
	"Backend/internal/models"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrRecalculationTargetChanged は、対象を選んだ後でセッションが再計算できなくなったことを示す。
// 履歴書・ES・面接のスコアが途中で反映された場合に返す。
var ErrRecalculationTargetChanged = errors.New("recalculation target is no longer eligible")

type UserWeightScoreRepository struct {
	db *gorm.DB
}

func NewUserWeightScoreRepository(db *gorm.DB) *UserWeightScoreRepository {
	return &UserWeightScoreRepository{db: db}
}

// SetScore スコアを絶対値で新規作成する。
// 呼び出し前に対象レコードが存在しないことを確認すること。
// スコアは 0〜100 に丸める。
func (r *UserWeightScoreRepository) SetScore(userID uint, sessionID, category string, absoluteScore int) error {
	return r.SetScoreWithPromptVersion(userID, sessionID, category, absoluteScore, "")
}

func (r *UserWeightScoreRepository) SetScoreWithPromptVersion(
	userID uint, sessionID, category string, absoluteScore int, promptVersion string,
) error {
	return r.setScoreWithSourceAndPromptVersion(
		userID, sessionID, category, absoluteScore, models.ScoreSourceChat, promptVersion,
	)
}

// SetScoreWithSource は由来（chat / assessment）を指定して SetScore する（#1678）。
func (r *UserWeightScoreRepository) SetScoreWithSource(userID uint, sessionID, category string, absoluteScore int, source string) error {
	return r.setScoreWithSourceAndPromptVersion(userID, sessionID, category, absoluteScore, source, "")
}

func (r *UserWeightScoreRepository) setScoreWithSourceAndPromptVersion(
	userID uint, sessionID, category string, absoluteScore int, source, promptVersion string,
) error {
	// 正典外のカテゴリ名を書かせない（#929）。
	// マッチングは正典キーで scoreMap を引くため、揺れた名前で保存されると
	// その行は永久に引かれず、代わりに中立50が使われて静かにスコアが希釈される。
	// 全ての書き込みがこのリポジトリを通るので、ここ1箇所で再発を止める。
	normalized, err := valueobject.ParseWeightCategory(category)
	if err != nil {
		return err
	}
	category = string(normalized)

	if absoluteScore < 0 {
		absoluteScore = 0
	}
	if absoluteScore > 100 {
		absoluteScore = 100
	}
	score := models.UserWeightScore{
		UserID:             userID,
		SessionID:          sessionID,
		WeightCategory:     category,
		Score:              absoluteScore,
		Source:             source,
		CalculationVersion: models.CurrentUserScoreCalculationVersion,
		PromptVersion:      promptVersion,
	}
	if err := fillOrganizationID(r.db, userID, &score.OrganizationID); err != nil {
		return err
	}
	return r.db.Create(&score).Error
}

// AddScore 既存スコアに差分を加算する。
// レコードが存在しない場合はエラーを返す。
// 加算結果は SQL 側で 0〜100 に丸め、競合時も範囲外にならないようにする。
func (r *UserWeightScoreRepository) AddScore(userID uint, sessionID, category string, delta int) error {
	return r.AddScoreWithPromptVersion(userID, sessionID, category, delta, "")
}

func (r *UserWeightScoreRepository) AddScoreWithPromptVersion(
	userID uint, sessionID, category string, delta int, promptVersion string,
) error {
	normalized, err := valueobject.ParseWeightCategory(category)
	if err != nil {
		return err
	}
	category = string(normalized)

	var score models.UserWeightScore
	if err := r.db.Where("user_id = ? AND session_id = ? AND weight_category = ?",
		userID, sessionID, category).First(&score).Error; err != nil {
		return err
	}
	// 既存スコアを混ぜる加算では、過去版の値が残るため計算版を更新しない。
	// プロンプト版は今回の評価に使った版を記録する（#1778）。
	updates := map[string]any{
		"score": gorm.Expr("GREATEST(0, LEAST(100, score + ?))", delta),
	}
	if promptVersion != "" {
		updates["prompt_version"] = promptVersion
	}
	return r.db.Model(&score).Updates(updates).Error
}

// FindByUserAndSession ユーザーとセッションに紐づく全スコアを取得
func (r *UserWeightScoreRepository) FindByUserAndSession(userID uint, sessionID string) ([]entity.UserWeightScore, error) {
	var ms []models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoresToEntities(ms), nil
}

// FindTopCategories トップNのカテゴリを取得
func (r *UserWeightScoreRepository) FindTopCategories(userID uint, sessionID string, limit int) ([]entity.UserWeightScore, error) {
	var ms []models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Order("score DESC").
		Limit(limit).
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoresToEntities(ms), nil
}

// FindByUserSessionAndCategory ユーザー、セッション、カテゴリで検索
func (r *UserWeightScoreRepository) FindByUserSessionAndCategory(userID uint, sessionID, category string) (*entity.UserWeightScore, error) {
	var m models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id = ? AND weight_category = ?", userID, sessionID, category).
		First(&m).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserWeightScoreToEntity(&m), nil
}

// CountByUserAndSession ユーザーとセッションに紐づくスコア数を取得
func (r *UserWeightScoreRepository) CountByUserAndSession(userID uint, sessionID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.UserWeightScore{}).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&count).Error
	return count, err
}

type UserScoreRecalculationTarget struct {
	ScoreID   uint
	UserID    uint
	SessionID string
}

type UserScoreRecalculationStats struct {
	EligibleSessions    uint
	UnsupportedSessions uint
	MaxScoreID          uint
	EligibleAnswers     uint
}

// RecalculationStats returns chat sessions whose scores are out of date. Sessions
// with cross-feature contributions are excluded because their aggregate cannot be
// reconstructed from chat messages alone.
func (r *UserWeightScoreRepository) RecalculationStats(version string) (UserScoreRecalculationStats, error) {
	var stats UserScoreRecalculationStats
	err := r.db.Raw(fmt.Sprintf(`
SELECT
  COALESCE(MAX(uws.id), 0) AS max_score_id,
  COUNT(DISTINCT CASE WHEN %[1]s
    THEN CONCAT(uws.user_id, ':', uws.session_id) END) AS eligible_sessions,
  COUNT(DISTINCT CONCAT(uws.user_id, ':', uws.session_id)) -
  COUNT(DISTINCT CASE WHEN %[1]s
    THEN CONCAT(uws.user_id, ':', uws.session_id) END) AS unsupported_sessions
FROM user_weight_scores uws
WHERE uws.calculation_version <> ?`, recalculationEligibleCondition()), version).Scan(&stats).Error
	if err != nil {
		return stats, err
	}
	err = r.db.Raw(fmt.Sprintf(`
SELECT COUNT(*)
FROM chat_messages cm
WHERE cm.role = 'user'
  AND EXISTS (
    SELECT 1 FROM user_weight_scores uws
    WHERE uws.user_id = cm.user_id AND uws.session_id = cm.session_id
      AND uws.calculation_version <> ?
      AND %s
  )`, recalculationEligibleCondition()), version).Scan(&stats.EligibleAnswers).Error
	return stats, err
}

// recalculationEligibleCondition は別名 uws のスコア行が再計算できるかを判定する条件を返す。
// 見積もりと実際の対象がずれると進捗と完了判定が狂うので、全クエリでこの条件を共有する。
func recalculationEligibleCondition() string {
	return fmt.Sprintf(`uws.source = 'chat'
  AND LEFT(uws.session_id, 10) <> 'interview-'
  AND EXISTS (
    SELECT 1 FROM chat_messages eligible_cm
    WHERE eligible_cm.user_id = uws.user_id AND eligible_cm.session_id = uws.session_id
  )
  AND NOT EXISTS (
    SELECT 1 FROM resume_documents rd
    JOIN resume_reviews rr ON rr.document_id = rd.id
    WHERE rd.user_id = uws.user_id AND rd.session_id = uws.session_id
      AND rr.score IS NOT NULL
  )
  AND NOT EXISTS (
    SELECT 1 FROM es_reviews er
    WHERE er.user_id = uws.user_id AND er.scored_at IS NOT NULL
      AND (%s OR %s)
  )
  AND NOT EXISTS (
    SELECT 1 FROM interview_sessions ins
    JOIN interview_reports ir ON ir.session_id = ins.id
    WHERE ins.user_id = uws.user_id AND ir.scores_applied_at IS NOT NULL
      AND (%s OR %s)
  )`,
		scoreSessionAtTime("uws", "er.scored_at"),
		scoreSessionTouchedAfter("uws", "er.scored_at"),
		scoreSessionAtTime("uws", "ir.scores_applied_at"),
		scoreSessionTouchedAfter("uws", "ir.scores_applied_at"),
	)
}

func (r *UserWeightScoreRepository) FindNextRecalculationTargets(version string, maxScoreID, cursor uint, limit int) ([]UserScoreRecalculationTarget, error) {
	var targets []UserScoreRecalculationTarget
	if limit <= 0 {
		return targets, nil
	}
	err := r.db.Raw(fmt.Sprintf(`
SELECT MIN(uws.id) AS score_id, uws.user_id, uws.session_id
FROM user_weight_scores uws
WHERE uws.calculation_version <> ?
  AND uws.id <= ?
  AND %s
GROUP BY uws.user_id, uws.session_id
HAVING MIN(uws.id) > ?
ORDER BY MIN(uws.id)
LIMIT ?`, recalculationEligibleCondition()), version, maxScoreID, cursor, limit).Scan(&targets).Error
	return targets, err
}

func scoreSessionAtTime(scoreAlias, appliedAt string) string {
	return fmt.Sprintf(`%s.session_id = (
  SELECT prior.session_id
  FROM user_weight_scores prior
  WHERE prior.user_id = %s.user_id
    AND prior.source <> 'assessment'
    AND LEFT(prior.session_id, 10) <> 'interview-'
    AND prior.updated_at <= %s
  ORDER BY prior.updated_at DESC, prior.id DESC
  LIMIT 1
)`, scoreAlias, scoreAlias, appliedAt)
}

// scoreSessionTouchedAfter は、反映時刻より前に作られ、反映時刻以降に更新された行が
// セッションにあるかを判定する条件を返す。
//
// 面接は反映時刻を確保してから行を更新するので、更新した行の updated_at は反映時刻より
// 後になる。全行が更新されたセッションは scoreSessionAtTime の候補から外れ、加点済みと
// 判定できない。誤って再計算すると加点が消えるので、迷う場合は対象外に倒す。
// 再計算で作り直した行は created_at も新しくなるので、この条件には当たらない。
func scoreSessionTouchedAfter(scoreAlias, appliedAt string) string {
	return fmt.Sprintf(`EXISTS (
  SELECT 1 FROM user_weight_scores touched
  WHERE touched.user_id = %[1]s.user_id
    AND touched.session_id = %[1]s.session_id
    AND touched.created_at <= %[2]s
    AND touched.updated_at >= %[2]s
)`, scoreAlias, appliedAt)
}

// ReplaceChatSessionScores atomically replaces a replayed chat session's scores.
//
// 再計算は LLM を何度も呼ぶので、対象を選んでから置換するまでに履歴書・ES・面接の
// スコアが反映されることがある。行をロックしてから対象の条件を確かめ直し、外れていれば
// ErrRecalculationTargetChanged を返す。確かめずに置換すると、反映された加点が消える。
//
// 置換前の行は jobID に紐づけて残し、RollbackRecalculatedSessions で元に戻せるようにする。
func (r *UserWeightScoreRepository) ReplaceChatSessionScores(jobID, userID uint, sessionID string, scores map[string]int, version string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEligibleChatSession(tx, userID, sessionID, version); err != nil {
			return err
		}

		var existing []models.UserWeightScore
		if err := tx.Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
			Find(&existing).Error; err != nil {
			return err
		}
		payload, err := json.Marshal(existing)
		if err != nil {
			return err
		}
		// 置換しても created_at / updated_at は元の行から引き継ぐ（B1）。再計算の時刻を打つと、
		// 「最新セッション」を updated_at で決める FindLatestByUser・FindLatestDiagnosisSessionID・
		// FindLatestScoresByUsers で、古いセッションが最新に昇格して表示とマッチングの基準が巻き戻る。
		// 元に無かったカテゴリは、元の行の最小の created_at と最大の updated_at を使う。
		byCategory, minCreated, maxUpdated := recalculationTimes(existing)
		if err := tx.Create(&models.UserScoreRecalculationSnapshot{
			JobID: jobID, UserID: userID, SessionID: sessionID, ScoresJSON: string(payload),
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
			Delete(&models.UserWeightScore{}).Error; err != nil {
			return err
		}
		for category, score := range scores {
			normalized, err := valueobject.ParseWeightCategory(category)
			if err != nil {
				return err
			}
			row := models.UserWeightScore{
				UserID: userID, SessionID: sessionID, Source: models.ScoreSourceChat,
				WeightCategory: string(normalized), Score: max(0, min(100, score)),
				CalculationVersion: version,
			}
			if prev, ok := byCategory[row.WeightCategory]; ok {
				row.CreatedAt, row.UpdatedAt = prev.CreatedAt, prev.UpdatedAt
			} else if len(existing) > 0 {
				row.CreatedAt, row.UpdatedAt = minCreated, maxUpdated
			}
			if len(existing) > 0 {
				row.OrganizationID = existing[0].OrganizationID
			} else if err := fillOrganizationID(tx, userID, &row.OrganizationID); err != nil {
				return err
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// recalculationTimes は置換前の行から、カテゴリ別の行・最小の created_at・最大の updated_at を返す。
// 置換で時刻を引き継ぐときと、ロールバックで「再計算の後に更新があったか」を判定するときに、
// 同じ規則を使う。
func recalculationTimes(rows []models.UserWeightScore) (map[string]models.UserWeightScore, time.Time, time.Time) {
	byCategory := make(map[string]models.UserWeightScore, len(rows))
	var minCreated, maxUpdated time.Time
	for _, r := range rows {
		byCategory[r.WeightCategory] = r
		if minCreated.IsZero() || r.CreatedAt.Before(minCreated) {
			minCreated = r.CreatedAt
		}
		if r.UpdatedAt.After(maxUpdated) {
			maxUpdated = r.UpdatedAt
		}
	}
	return byCategory, minCreated, maxUpdated
}

// changedSinceRecalculation は、再計算で書いた行の後に更新があったかを返す（B5）。
//
// 置換は元の行の時刻を引き継ぐので（recalculationTimes）、updated_at が置換時に書いた値と
// 違えば、その後にライブの採点（AddScore）や新しいカテゴリの回答（SetScore）があった。
// 計算版が違う行も、別の経路で書き直されている。updated_at > created_at の比較は、時刻を
// 引き継ぐと再計算前の更新でも真になるので使わない。
func changedSinceRecalculation(current, snapshot []models.UserWeightScore, version string) bool {
	if len(snapshot) == 0 {
		// 置換前の行が無ければ時刻を引き継いでいないので、作成後に更新されたかだけを見る
		for _, r := range current {
			if r.CalculationVersion != version || !r.UpdatedAt.Equal(r.CreatedAt) {
				return true
			}
		}
		return false
	}
	byCategory, _, maxUpdated := recalculationTimes(snapshot)
	for _, r := range current {
		if r.CalculationVersion != version {
			return true
		}
		expected := maxUpdated
		if prev, ok := byCategory[r.WeightCategory]; ok {
			expected = prev.UpdatedAt
		}
		// DB の精度（秒またはミリ秒）で丸めて比べる
		if !r.UpdatedAt.Truncate(time.Second).Equal(expected.Truncate(time.Second)) {
			return true
		}
	}
	return false
}

// MarkChatSessionCalculationVersion は、採点できる回答が無かったセッションの計算版だけを
// 現行にする。スコアは置き換えない。
//
// 計算版が古いままだと、次のジョブで対象の先頭に戻る。履歴が変わらなくても、短文・境界値の
// LLM 評価がジョブのたびに繰り返される。現行の計算式で再計算した結果が「採点できる回答なし」
// なので、既存の値は残したまま計算版だけを進める。置換と同じく、ロックしてから対象の条件を
// 確かめ直し、外れていれば ErrRecalculationTargetChanged を返す。
//
// updated_at は動かさない。最新セッションの判定（FindLatestByUser など）と、履歴書・ES・面接の
// 加点の判定（scoreSessionAtTime / scoreSessionTouchedAfter）が updated_at を見るため。
// スナップショットは残さないので、ロールバックしても計算版は戻らない（スコアは変わっていない）。
func (r *UserWeightScoreRepository) MarkChatSessionCalculationVersion(userID uint, sessionID, version string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := lockEligibleChatSession(tx, userID, sessionID, version); err != nil {
			return err
		}
		return tx.Model(&models.UserWeightScore{}).
			Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
			UpdateColumn("calculation_version", version).Error
	})
}

// lockEligibleChatSession はセッションのチャットスコア行をロックし、まだ再計算の対象かを
// 確かめ直す。外れていれば ErrRecalculationTargetChanged を返す。
func lockEligibleChatSession(tx *gorm.DB, userID uint, sessionID, version string) error {
	var lockedIDs []uint
	if err := tx.Model(&models.UserWeightScore{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
		Pluck("id", &lockedIDs).Error; err != nil {
		return err
	}
	var eligible int64
	if err := tx.Raw(fmt.Sprintf(`
SELECT COUNT(*)
FROM user_weight_scores uws
WHERE uws.user_id = ? AND uws.session_id = ? AND uws.calculation_version <> ?
  AND %s`, recalculationEligibleCondition()), userID, sessionID, version).Scan(&eligible).Error; err != nil {
		return err
	}
	if eligible == 0 {
		return ErrRecalculationTargetChanged
	}
	return nil
}

// ChatSessionScoreMap はセッションのチャットスコアをカテゴリ別に返す。
func (r *UserWeightScoreRepository) ChatSessionScoreMap(userID uint, sessionID string) (map[string]int, error) {
	var rows []models.UserWeightScore
	if err := r.db.Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	scores := make(map[string]int, len(rows))
	for _, row := range rows {
		scores[row.WeightCategory] = row.Score
	}
	return scores, nil
}

// RecalculationRollbackResult は1回のロールバックの結果。
type RecalculationRollbackResult struct {
	// Restored は元に戻したセッション。企業マッチングの作り直しに使う。
	Restored []UserScoreRecalculationTarget
	// Skipped は再計算の後に更新があったので戻さなかったセッションの数。
	Skipped int
	// LastSnapshotID は今回見た最後のスナップショットの ID。次の呼び出しの afterID に渡す。
	LastSnapshotID uint
}

// RollbackRecalculatedSessions はジョブのスナップショットを、afterID より後から limit 件まで元に戻す。
//
// 戻したセッションのスナップショットだけを消す（B5）。戻さなかったセッション（再計算の後に更新が
// あった）の控えは残すので、「処理した行を消す」ことで進むページングはできない。ID のカーソルで進め、
// 呼び出し側は LastSnapshotID を次の afterID に渡す。
func (r *UserWeightScoreRepository) RollbackRecalculatedSessions(jobID uint, version string, limit int, afterID uint) (RecalculationRollbackResult, error) {
	result := RecalculationRollbackResult{LastSnapshotID: afterID}
	var snapshots []models.UserScoreRecalculationSnapshot
	if err := r.db.Where("job_id = ? AND id > ?", jobID, afterID).Order("id").Limit(limit).Find(&snapshots).Error; err != nil {
		return result, err
	}
	for _, snapshot := range snapshots {
		result.LastSnapshotID = snapshot.ID
		restored, err := r.rollbackSnapshot(snapshot, version)
		if err != nil {
			return result, err
		}
		if restored {
			result.Restored = append(result.Restored, UserScoreRecalculationTarget{UserID: snapshot.UserID, SessionID: snapshot.SessionID})
		} else {
			result.Skipped++
		}
	}
	return result, nil
}

// rollbackSnapshot は1セッションを元に戻す。再計算の後にチャット回答や履歴書・ES・面接の
// 反映があったセッションは、戻すとその更新が消えるので戻さない。戻さなかったスナップショットは
// 消さない（唯一の控えなので）。
func (r *UserWeightScoreRepository) rollbackSnapshot(snapshot models.UserScoreRecalculationSnapshot, version string) (bool, error) {
	var rows []models.UserWeightScore
	if err := json.Unmarshal([]byte(snapshot.ScoresJSON), &rows); err != nil {
		return false, fmt.Errorf("snapshot %d: %w", snapshot.ID, err)
	}
	restored := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		scope := tx.Model(&models.UserWeightScore{}).
			Where("user_id = ? AND session_id = ? AND source = ?", snapshot.UserID, snapshot.SessionID, models.ScoreSourceChat)
		var current []models.UserWeightScore
		if err := scope.Session(&gorm.Session{}).Clauses(clause.Locking{Strength: "UPDATE"}).Order("id").Find(&current).Error; err != nil {
			return err
		}
		if len(current) == 0 || changedSinceRecalculation(current, rows, version) {
			return nil
		}
		lockedIDs := make([]uint, len(current))
		for i, r := range current {
			lockedIDs[i] = r.ID
		}
		if err := tx.Where("id IN ?", lockedIDs).Delete(&models.UserWeightScore{}).Error; err != nil {
			return err
		}
		for i := range rows {
			rows[i].ID = 0
			if err := tx.Create(&rows[i]).Error; err != nil {
				return err
			}
		}
		restored = true
		return tx.Delete(&models.UserScoreRecalculationSnapshot{}, snapshot.ID).Error
	})
	return restored, err
}

// FindLatestByUser ユーザーの最新セッションのスコアを取得する。
// 面接スナップショット(interview-*)よりチャット診断セッションを優先する。
// 適性検査の行は「最新セッション」の候補にしない（#1678。合流はマッチング側で行う）。
func (r *UserWeightScoreRepository) FindLatestByUser(userID uint) ([]entity.UserWeightScore, error) {
	var latest models.UserWeightScore
	err := r.db.Where("user_id = ? AND source <> ?", userID, models.ScoreSourceAssessment).
		Order("CASE WHEN session_id LIKE 'interview-%' THEN 1 ELSE 0 END ASC, updated_at DESC, id DESC").
		First(&latest).Error
	if err != nil {
		return nil, err
	}
	return r.FindByUserAndSession(userID, latest.SessionID)
}

// FindLatestDiagnosisSessionID はマッチング対象に使うチャット診断セッションを返す。
// 面接スナップショットは除外。無い場合は gorm.ErrRecordNotFound。
func (r *UserWeightScoreRepository) FindLatestDiagnosisSessionID(userID uint) (string, error) {
	var latest models.UserWeightScore
	err := r.db.Where("user_id = ? AND session_id NOT LIKE ? AND source <> ?", userID, "interview-%", models.ScoreSourceAssessment).
		Order("updated_at DESC, id DESC").
		First(&latest).Error
	if err != nil {
		return "", err
	}
	return latest.SessionID, nil
}

// FindLatestScoresByUsers は複数ユーザーの「最新セッションのスコア」を1クエリで返す（#1027）。
//
// 教員向け一覧は担当生徒ぶん繰り返し引くため、FindLatestByUser
// (1ユーザーあたり2クエリ)をループすると即 N+1 になる。
// ウィンドウ関数で「ユーザーごとに最新の1行」を決め、その session_id の
// 行だけを拾う。戻り値は user_id -> (正典カテゴリ名 -> スコア)。
//
// 「最新」はチャット診断を優先し、同優先内では updated_at / id 降順。
func (r *UserWeightScoreRepository) FindLatestScoresByUsers(userIDs []uint) (map[uint]map[string]float64, error) {
	result := map[uint]map[string]float64{}
	if len(userIDs) == 0 {
		return result, nil
	}

	type row struct {
		UserID         uint
		WeightCategory string
		Score          int
	}
	var rows []row

	const q = `
WITH latest AS (
  SELECT user_id, session_id,
         ROW_NUMBER() OVER (
           PARTITION BY user_id
           ORDER BY
             CASE WHEN session_id LIKE 'interview-%' THEN 1 ELSE 0 END ASC,
             updated_at DESC,
             id DESC
         ) AS rn
  FROM user_weight_scores
  WHERE user_id IN (?) AND source <> 'assessment'
)
SELECT s.user_id, s.weight_category, s.score
FROM user_weight_scores s
JOIN latest l ON l.user_id = s.user_id AND l.session_id = s.session_id AND l.rn = 1
WHERE s.user_id IN (?)`

	if err := r.db.Raw(q, userIDs, userIDs).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, x := range rows {
		if result[x.UserID] == nil {
			result[x.UserID] = map[string]float64{}
		}
		result[x.UserID][x.WeightCategory] = float64(x.Score)
	}
	return result, nil
}
