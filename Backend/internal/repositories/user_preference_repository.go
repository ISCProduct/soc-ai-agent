package repositories

import (
	"strings"
	"time"

	"Backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// UserPreferenceRepository は学生の希望条件を扱う（#1094）。
type UserPreferenceRepository struct {
	db *gorm.DB
}

func NewUserPreferenceRepository(db *gorm.DB) *UserPreferenceRepository {
	return &UserPreferenceRepository{db: db}
}

// FindByUserID は希望条件を返す。未設定なら nil, nil。
func (r *UserPreferenceRepository) FindByUserID(userID uint) (*models.UserPreference, error) {
	var m models.UserPreference
	err := r.db.Where("user_id = ?", userID).First(&m).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// Upsert は希望条件を作成または更新する（1学生1レコード）。
func (r *UserPreferenceRepository) Upsert(pref *models.UserPreference) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"desired_industry_id", "desired_location", "note", "updated_at",
		}),
	}).Create(pref).Error
}

// SetScoutVisibility はスカウト公開同意フラグのみを更新する（#1094）。
// entity 全体を保存すると他フィールドを巻き戻す恐れがあるため、対象カラムだけを更新する。
func (r *UserPreferenceRepository) SetScoutVisibility(userID uint, allow bool) error {
	return r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("allow_scout_visibility", allow).Error
}

// GetScoutVisibility は学生本人が設定した公開同意フラグをそのまま返す。
// 本人の設定画面へ現在値を返すためのもので、実際に企業へ公開されるかは
// IsScoutVisible で判定すること。
func (r *UserPreferenceRepository) GetScoutVisibility(userID uint) (bool, error) {
	var allow bool
	err := r.db.Model(&models.User{}).
		Where("id = ?", userID).
		Select("allow_scout_visibility").Scan(&allow).Error
	return allow, err
}

// SetTrainingUse は、面接データを学習に使うことへの同意フラグだけを更新し（#2167）、
// その操作を同意の履歴（training_consent_events）に1行残す（#2168）。
// SetScoutVisibility と同じ理由で、users は対象カラムだけを更新する。
//
// 更新と履歴の追加は同じトランザクションで行う。片方だけ成功すると、
// 「同意したのに記録が無い」「記録はあるのに値が変わっていない」状態になり、後から説明できない。
// 同じ値への変更も、本人の明示的な操作なので記録する。
// textVersion は、操作したときに画面に出していた同意の文言の版（呼び出し側で今の版と一致することを確かめる）。
//
// created_at はアプリの時計ではなく DB の時計（NOW(3)）で入れる。API のタスクが複数あると、
// タスクごとの時計のずれで履歴の日時の並びが操作の順と食い違うため。最後の操作は id で決める
// （GetTrainingConsent）。
func (r *UserPreferenceRepository) SetTrainingUse(userID uint, allow bool, textVersion string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).
			Where("id = ?", userID).
			Update("allow_training_use", allow).Error; err != nil {
			return err
		}
		return tx.Exec(
			"INSERT INTO `training_consent_events` (`user_id`,`allow_training_use`,`consent_text_version`,`created_at`) VALUES (?,?,?,NOW(3))",
			userID, allow, textVersion,
		).Error
	})
}

// TrainingConsentState は、学習利用の同意の現在値と、最後に変更した日時（#2168）。
// ChangedAt は同意の履歴の最後の行の日時で、行が無ければ nil（一度も変更していない初期値の「同意しない」）。
// 履歴の記録を始める前から同意していた人は、マイグレーション（000075）で作った行の日時（適用時刻）になる。
type TrainingConsentState struct {
	AllowTrainingUse bool
	ChangedAt        *time.Time
}

// GetTrainingConsent は、本人の同意の現在値と最後に変更した日時を、1つのクエリで読む（#2168）。
//
// 値と履歴を別々のクエリで読むと、その間にほかのタブ・端末の保存が挟まったとき、
// 「新しい値」と「古い日時」（またはその逆）を組み合わせて返してしまう。1つの文なら同じスナップショットで読める。
// 最後の行は created_at ではなく id（AUTO_INCREMENT、コミットの順に近い）の最大で決める。
// users.updated_at はほかの更新でも上書きされるので使わない。
// ユーザーが無ければ「同意しない」・日時なしを返す（以前の GetTrainingUse と同じ。認証済みの本人なので通常は起きない）。
// 書き出しの対象かどうか（退会していないかなど）は、書き出し側で判定する。
func (r *UserPreferenceRepository) GetTrainingConsent(userID uint) (TrainingConsentState, error) {
	var rows []struct {
		AllowTrainingUse bool
		ChangedAt        *time.Time
	}
	err := r.db.Raw(
		"SELECT u.allow_training_use, e.created_at AS changed_at FROM `users` u "+
			"LEFT JOIN `training_consent_events` e ON e.id = "+
			"(SELECT MAX(id) FROM `training_consent_events` WHERE user_id = u.id) "+
			"WHERE u.id = ?",
		userID,
	).Scan(&rows).Error
	if err != nil || len(rows) == 0 {
		return TrainingConsentState{}, err
	}
	return TrainingConsentState{AllowTrainingUse: rows[0].AllowTrainingUse, ChangedAt: rows[0].ChangedAt}, nil
}

// IsScoutVisible は実際に企業へ公開してよい状態かを返す（#1094）。
//
// 同意フラグだけでは足りない。退会しても allow_scout_visibility は 1 のまま
// 残るため、フラグのみで判定すると退会済み学生をベクトル化し直してしまう。
// 判定は検索側と同じ StudentSearchRepository.IsVisible へ委ね、
// 「企業に公開してよい学生」の定義を一箇所に保つ。
func (r *UserPreferenceRepository) IsScoutVisible(userID uint) (bool, error) {
	return NewStudentSearchRepository(r.db).IsVisible(userID)
}

// ScoutProfileText は同意済み学生のベクトル化対象テキストを組み立てる（#1094）。
// 対象は学生が自ら入力した資格・希望条件のみ。履歴書全文や面接発話は
// 公開同意の範囲が別途整理されるまで含めない。
func (r *UserPreferenceRepository) ScoutProfileText(userID uint) (string, error) {
	var row struct {
		CertificationsAcquired   string
		CertificationsInProgress string
		DesiredLocation          string
		Note                     string
		IndustryName             string
	}
	err := r.db.Table("users AS u").
		Joins("LEFT JOIN user_preferences AS up ON up.user_id = u.id").
		Joins("LEFT JOIN industries AS i ON i.id = up.desired_industry_id").
		Where("u.id = ?", userID).
		Select(`u.certifications_acquired, u.certifications_in_progress,
			COALESCE(up.desired_location, '') AS desired_location,
			COALESCE(up.note, '') AS note,
			COALESCE(i.name, '') AS industry_name`).
		Scan(&row).Error
	if err != nil {
		return "", err
	}
	parts := []string{}
	for _, p := range [][2]string{
		{"取得資格", row.CertificationsAcquired},
		{"勉強中の資格", row.CertificationsInProgress},
		{"希望業界", row.IndustryName},
		{"希望勤務地", row.DesiredLocation},
		{"自己PR", row.Note},
	} {
		if v := strings.TrimSpace(p[1]); v != "" {
			parts = append(parts, p[0]+": "+v)
		}
	}
	return strings.Join(parts, "\n"), nil
}
