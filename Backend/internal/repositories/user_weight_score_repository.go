package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/domain/valueobject"
	"Backend/internal/models"

	"gorm.io/gorm"
)

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
		UserID:         userID,
		SessionID:      sessionID,
		WeightCategory: category,
		Score:          absoluteScore,
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

// FindLatestByUser ユーザーの最新セッションのスコアを取得する
func (r *UserWeightScoreRepository) FindLatestByUser(userID uint) ([]entity.UserWeightScore, error) {
	// 最新の session_id を特定
	var latest models.UserWeightScore
	err := r.db.Where("user_id = ?", userID).
		Order("updated_at DESC").
		First(&latest).Error
	if err != nil {
		return nil, err
	}
	return r.FindByUserAndSession(userID, latest.SessionID)
}

// FindLatestScoresByUsers は複数ユーザーの「最新セッションのスコア」を1クエリで返す（#1027）。
//
// 教員向け一覧は担当生徒ぶん繰り返し引くため、FindLatestByUser
// (1ユーザーあたり2クエリ)をループすると即 N+1 になる。
// ウィンドウ関数で「ユーザーごとに最新の1行」を決め、その session_id の
// 行だけを拾う。戻り値は user_id -> (正典カテゴリ名 -> スコア)。
//
// 「最新」は updated_at 降順、同値は id 降順で決める。
// updated_at を使うのは既存の FindLatestByUser と揃えるため。
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

	// latest: ユーザーごとに最新1行を選び、その session_id を確定させる。
	// scores: 同じ (user_id, session_id) の全カテゴリを取る。
	const q = `
WITH latest AS (
  SELECT user_id, session_id,
         ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY updated_at DESC, id DESC) AS rn
  FROM user_weight_scores
  WHERE user_id IN (?)
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
