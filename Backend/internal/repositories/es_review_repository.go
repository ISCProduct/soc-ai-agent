package repositories

import (
	"errors"
	"fmt"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
)

// ESReviewRepository は ES添削の履歴（#1531）。
type ESReviewRepository struct {
	db *gorm.DB
}

func NewESReviewRepository(db *gorm.DB) *ESReviewRepository {
	return &ESReviewRepository{db: db}
}

func (r *ESReviewRepository) Create(review *models.ESReview) error {
	return r.db.Create(review).Error
}

// FindPreviousByQuestion は同一設問の直前の添削を1件返す。無ければ (nil, nil)。
//
// excludeID には今回保存した行を渡す。保存の後に引くので、渡さないと自分自身が
// 「前回」になって差分が常にゼロになる。
func (r *ESReviewRepository) FindPreviousByQuestion(userID uint, questionKey string, excludeID uint) (*models.ESReview, error) {
	var review models.ESReview
	err := r.db.Where("user_id = ? AND question_key = ? AND id < ?", userID, questionKey, excludeID).
		Order("id DESC").
		First(&review).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &review, nil
}

// HasScoredSameText は本人が同じ本文で既にスコアへ反映済みかを返す。
//
// 「反映済みか」で引くのが要点。保存済みかで引くと、診断セッションが無くて反映
// できなかった添削が「計上済み」と誤判定され、その本文が以後永久にスコアへ乗らない。
// excludeID には今回保存した行を渡す。渡さないと自分自身に当たって常に true になる。
func (r *ESReviewRepository) HasScoredSameText(userID uint, textHash string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&models.ESReview{}).
		Where("user_id = ? AND es_text_hash = ? AND id < ? AND scored_at IS NOT NULL",
			userID, textHash, excludeID).
		Limit(1).
		Count(&count).Error
	return count > 0, err
}

// LastScoredAt は本人が直近でスコアへ反映した時刻。まだ無ければ nil。
//
// 反映の頻度に上限を置くために使う。本文の正規化をどれだけ強めても、
// 一語書き換えれば別の本文になるので、内容の判定だけでは積み上げを止められない。
func (r *ESReviewRepository) LastScoredAt(userID, excludeID uint) (*time.Time, error) {
	var row struct{ ScoredAt *time.Time }
	err := r.db.Model(&models.ESReview{}).
		Select("MAX(scored_at) AS scored_at").
		Where("user_id = ? AND id < ? AND scored_at IS NOT NULL", userID, excludeID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.ScoredAt, nil
}

// scoringLockWaitSeconds は同一ユーザーの反映待ちの上限。超えたら反映を見送る
// （待ち続けるより、二重に積まない方を選ぶ）。
const scoringLockWaitSeconds = 5

// WithUserScoringLock は同一ユーザーの「反映してよいかの判定→反映→記録」を直列化して fn を実行する。
//
// 判定と記録が別のクエリなので、並行に2本来ると両方が「未反映」と見て二重に加点する
// （ダブルクリック・複数タブ・リトライ・本文を少し変えた並列送信。24時間の間隔制限も効かない）。
// 行ロックでなく GET_LOCK にしているのは、反映側の書き込みが users を外部キーで参照するため、
// users 行を排他ロックしたまま反映すると自分自身と待ち合わせてしまうから。
// 取得できなかったときは fn を呼ばず false を返す。
func (r *ESReviewRepository) WithUserScoringLock(userID uint, fn func()) (bool, error) {
	name := fmt.Sprintf("es_review_score:%d", userID)
	acquired := false
	err := r.db.Connection(func(tx *gorm.DB) error {
		var got *int
		if err := tx.Raw("SELECT GET_LOCK(?, ?)", name, scoringLockWaitSeconds).Scan(&got).Error; err != nil {
			return err
		}
		if got == nil || *got != 1 {
			return nil
		}
		acquired = true
		// 解放に失敗しても接続が閉じれば外れる。fn の結果は妨げない。
		defer tx.Exec("SELECT RELEASE_LOCK(?)", name)
		fn()
		return nil
	})
	return acquired, err
}

// MarkScored はスコアへ反映したことを記録する。
func (r *ESReviewRepository) MarkScored(id uint, at time.Time) error {
	return r.db.Model(&models.ESReview{}).Where("id = ?", id).Update("scored_at", at).Error
}

// ListByUser は学校で絞らずに添削履歴を新しい順に返す。
//
// 担当校を持たない無制限管理者の経路でだけ使う（学校で絞る経路は ListForSchoolStudent）。
// 全テナント横断で見えるのは ListStudentsPaged の前例どおりだが、退会済みは除く。
// 退会の猶予期間中のデータは閲覧不可が方針（docs/wiki/data-privacy.md）。
func (r *ESReviewRepository) ListByUser(userID uint, limit int) ([]models.ESReview, error) {
	if limit <= 0 {
		limit = 20
	}
	var reviews []models.ESReview
	err := r.db.Table("es_reviews AS e").
		Select("e.*").
		Joins("JOIN users u ON u.id = e.user_id").
		Where("e.user_id = ? AND u.withdrawn_at IS NULL", userID).
		// 「新しい順」は作成時刻で決める。id を第2キーにするのは、同一時刻でも
		// 並びが揺れないようにするため。
		Order("e.created_at DESC, e.id DESC").
		Limit(limit).
		Find(&reviews).Error
	return reviews, err
}

// ListForSchoolStudent は教員向けに、担当校の在籍学生1名の添削履歴を新しい順に返す。
//
// 学校だけで絞ると他学園の学生が混ざる。users.school_id はプロフィールの学校名
// （本人の自己申告）から解決されるため、学校名を他学園の校名に書き替えた学生が
// その学校の履歴として見えてしまう。学校の学園と本人の学園の一致を要求し、
// さらに保存時点の学園（es_reviews.organization_id）とも突き合わせる。
//
// 在籍中の student だけを対象にするのは、教員向けの一覧（ListStudentsPaged）と
// 見える範囲を揃えるため。一覧に出ない学生の履歴だけ見えるのはおかしい。
func (r *ESReviewRepository) ListForSchoolStudent(studentUserID, schoolID uint, limit int) ([]models.ESReview, error) {
	if limit <= 0 {
		limit = 50
	}
	var reviews []models.ESReview
	err := r.db.Table("es_reviews AS e").
		Select("e.*").
		Joins("JOIN users u ON u.id = e.user_id").
		Joins("JOIN schools s ON s.id = u.school_id AND s.organization_id = u.organization_id").
		Where("e.user_id = ? AND u.school_id = ? AND e.organization_id = u.organization_id", studentUserID, schoolID).
		Where("u.role = ? AND u.is_guest = ? AND u.is_admin = ? AND u.withdrawn_at IS NULL",
			models.UserRoleStudent, false, false).
		// 「新しい順」は作成時刻で決める。id は同一時刻での揺れ止め。
		Order("e.created_at DESC, e.id DESC").
		Limit(limit).
		Find(&reviews).Error
	return reviews, err
}
