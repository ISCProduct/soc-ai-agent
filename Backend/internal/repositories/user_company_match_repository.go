package repositories

import (
	"Backend/domain/entity"
	"Backend/domain/mapper"
	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserCompanyMatchRepository struct {
	db *gorm.DB
}

func NewUserCompanyMatchRepository(db *gorm.DB) *UserCompanyMatchRepository {
	return &UserCompanyMatchRepository{db: db}
}

// upsertAssignments は衝突時に上書きする列（ON DUPLICATE KEY UPDATE の右辺）。
// is_viewed / is_favorited / is_applied はユーザー操作の結果なので含めない（再計算で消さない）。
var upsertAssignments = clause.AssignmentColumns([]string{
	"job_position_id",
	"match_score",
	"technical_match",
	"teamwork_match",
	"leadership_match",
	"creativity_match",
	"stability_match",
	"growth_match",
	"work_life_match",
	"challenge_match",
	"detail_match",
	"communication_match",
	"matched_axis_count",
	"match_reason",
	"updated_at",
})

// CreateOrUpdate マッチング結果を作成または更新
func (r *UserCompanyMatchRepository) CreateOrUpdate(match *entity.UserCompanyMatch) error {
	_, err := r.CreateOrUpdateBatch([]*entity.UserCompanyMatch{match})
	return err
}

// CreateOrUpdateBatch は同一 user/session のマッチを一括 upsert する。
//
// 一意キー uniq_user_session_company で衝突を検出し、1回の INSERT ... ON DUPLICATE KEY UPDATE
// で作成と更新をまとめる（#1166）。既存の閲覧/お気に入り/応募フラグは upsertAssignments に
// 含めないことで保持される。保存に失敗した場合はエラーを呼び出し元へ返す。
func (r *UserCompanyMatchRepository) CreateOrUpdateBatch(matches []*entity.UserCompanyMatch) (int, error) {
	if len(matches) == 0 {
		return 0, nil
	}
	userID := matches[0].UserID
	sessionID := matches[0].SessionID

	rows := make([]*models.UserCompanyMatch, 0, len(matches))
	for _, match := range matches {
		if match == nil || match.UserID != userID || match.SessionID != sessionID {
			continue
		}
		m := mapper.UserCompanyMatchFromEntity(match)
		// 既存行のIDは分からないので主キーは空にし、衝突検出は一意キーに任せる
		m.ID = 0
		rows = append(rows, m)
	}
	if len(rows) == 0 {
		return 0, nil
	}

	// MySQL ドライバは OnConflict.Columns を無視して ON DUPLICATE KEY UPDATE を書くだけなので
	// 衝突検出は一意キー uniq_user_session_company（migration 000023）に依存する。
	// 将来この表に別の一意キーを足すと、そちらでも衝突して同じ列が更新される点に注意。
	err := r.db.Clauses(clause.OnConflict{DoUpdates: upsertAssignments}).
		CreateInBatches(rows, 100).Error
	if err != nil {
		return 0, err
	}
	return len(rows), nil
}

// CompanyHasWeightProfileSQL は「会社単位プロファイルを持つ企業」に限定する SQL 条件を返す（#1380）。
//
// プロファイルが無い企業は CalculateMatching が計算対象から外す（matching_service.go）。
// だが既に保存済みの user_company_matches は残るため、読み出し側でも外さないと
// デプロイ前にデフォルト重み（全軸50）で作られた行や、プロファイル削除前に作られた行が
// 推薦一覧・メールレポート・教員の低マッチ集計に古いスコアのまま出続ける。
//
// 行は消さない。is_viewed / is_favorited / is_applied はユーザー操作の結果で、
// プロファイル生成をやり直せばそのまま復帰できる。削除すると復帰できない。
//
// companyIDColumn は呼び出し元がコード内リテラルで渡す列名（外部入力を渡さないこと）。
func CompanyHasWeightProfileSQL(companyIDColumn string) string {
	return "EXISTS (SELECT 1 FROM company_weight_profiles cwp" +
		" WHERE cwp.company_id = " + companyIDColumn +
		" AND cwp.job_position_id IS NULL)"
}

// FindTopMatchesByUserAndSession マッチング度の高い順に企業を取得
func (r *UserCompanyMatchRepository) FindTopMatchesByUserAndSession(
	userID uint, sessionID string, limit int,
) ([]*entity.UserCompanyMatch, error) {
	var ms []*models.UserCompanyMatch
	err := r.db.Where("user_id = ? AND session_id = ?", userID, sessionID).
		Where(CompanyHasWeightProfileSQL("user_company_matches.company_id")).
		Order("match_score DESC").
		Limit(limit).
		Preload("Company").
		Preload("JobPosition").
		Find(&ms).Error

	if err != nil {
		return nil, err
	}

	result := make([]*entity.UserCompanyMatch, len(ms))
	for i, m := range ms {
		result[i] = mapper.UserCompanyMatchToEntity(m)
	}
	return result, nil
}

// FindByID IDでマッチング結果を取得
func (r *UserCompanyMatchRepository) FindByID(id uint) (*entity.UserCompanyMatch, error) {
	var m models.UserCompanyMatch
	err := r.db.Preload("Company").
		Preload("JobPosition").
		First(&m, id).Error
	if err != nil {
		return nil, err
	}
	return mapper.UserCompanyMatchToEntity(&m), nil
}

// MarkAsViewed 閲覧済みにする
func (r *UserCompanyMatchRepository) MarkAsViewed(matchID uint) error {
	return r.db.Model(&models.UserCompanyMatch{}).
		Where("id = ?", matchID).
		Update("is_viewed", true).Error
}

// ToggleFavorite お気に入りをトグル
func (r *UserCompanyMatchRepository) ToggleFavorite(matchID uint) error {
	var match models.UserCompanyMatch
	if err := r.db.First(&match, matchID).Error; err != nil {
		return err
	}
	return r.db.Model(&match).Update("is_favorited", !match.IsFavorited).Error
}

// MarkAsApplied 応募済みにする
func (r *UserCompanyMatchRepository) MarkAsApplied(matchID uint) error {
	return r.db.Model(&models.UserCompanyMatch{}).
		Where("id = ?", matchID).
		Update("is_applied", true).Error
}

// FindFavoritesByUser ユーザーのお気に入り企業を取得
func (r *UserCompanyMatchRepository) FindFavoritesByUser(userID uint, sessionID string) ([]*entity.UserCompanyMatch, error) {
	var ms []*models.UserCompanyMatch
	err := r.db.Where("user_id = ? AND session_id = ? AND is_favorited = ?", userID, sessionID, true).
		Order("match_score DESC").
		Preload("Company").
		Preload("JobPosition").
		Find(&ms).Error
	if err != nil {
		return nil, err
	}
	result := make([]*entity.UserCompanyMatch, len(ms))
	for i, m := range ms {
		result[i] = mapper.UserCompanyMatchToEntity(m)
	}
	return result, nil
}

// GetMatchStatistics マッチング統計情報を取得
func (r *UserCompanyMatchRepository) GetMatchStatistics(userID uint, sessionID string) (map[string]any, error) {
	var result struct {
		TotalMatches   int64
		ViewedCount    int64
		FavoritedCount int64
		AppliedCount   int64
		AvgMatchScore  float64
	}

	err := r.db.Model(&models.UserCompanyMatch{}).
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Count(&result.TotalMatches).Error
	if err != nil {
		return nil, err
	}

	r.db.Model(&models.UserCompanyMatch{}).
		Where("user_id = ? AND session_id = ? AND is_viewed = ?", userID, sessionID, true).
		Count(&result.ViewedCount)

	r.db.Model(&models.UserCompanyMatch{}).
		Where("user_id = ? AND session_id = ? AND is_favorited = ?", userID, sessionID, true).
		Count(&result.FavoritedCount)

	r.db.Model(&models.UserCompanyMatch{}).
		Where("user_id = ? AND session_id = ? AND is_applied = ?", userID, sessionID, true).
		Count(&result.AppliedCount)

	r.db.Model(&models.UserCompanyMatch{}).
		Select("AVG(match_score)").
		Where("user_id = ? AND session_id = ?", userID, sessionID).
		Scan(&result.AvgMatchScore)

	stats := map[string]any{
		"total_matches":   result.TotalMatches,
		"viewed_count":    result.ViewedCount,
		"favorited_count": result.FavoritedCount,
		"applied_count":   result.AppliedCount,
		"avg_match_score": result.AvgMatchScore,
	}

	return stats, nil
}
