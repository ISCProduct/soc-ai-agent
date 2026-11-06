package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/domain/valueobject"
	"Backend/internal/models"
	"errors"
	"fmt"

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
	return r.SetScoreWithSource(userID, sessionID, category, absoluteScore, models.ScoreSourceChat)
}

// SetScoreWithSource は由来（chat / assessment）を指定して SetScore する（#1678）。
func (r *UserWeightScoreRepository) SetScoreWithSource(userID uint, sessionID, category string, absoluteScore int, source string) error {
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
	return r.db.Model(&score).Update(
		"score",
		gorm.Expr("GREATEST(0, LEAST(100, score + ?))", delta),
	).Error
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
func (r *UserWeightScoreRepository) ReplaceChatSessionScores(userID uint, sessionID string, scores map[string]int, version string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
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

		var existing models.UserWeightScore
		err := tx.Where("user_id = ? AND session_id = ? AND source = ?", userID, sessionID, models.ScoreSourceChat).
			First(&existing).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
			if existing.ID != 0 {
				row.OrganizationID = existing.OrganizationID
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
