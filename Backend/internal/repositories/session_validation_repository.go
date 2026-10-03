package repositories

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"errors"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SessionValidationRepository struct {
	db *gorm.DB
}

func NewSessionValidationRepository(db *gorm.DB) *SessionValidationRepository {
	return &SessionValidationRepository{db: db}
}

// ClaimSessionOwnership は session_id の初回所有者を原子的に確定する。
// 競合時には一方だけが owner を獲得し、他方は forbidden を返す。
func (r *SessionValidationRepository) ClaimSessionOwnership(sessionID string, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing models.SessionValidation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("session_id = ?", sessionID).
			First(&existing).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			newRow := models.SessionValidation{
				SessionID:          sessionID,
				UserID:            &userID,
				InvalidAnswerCount: 0,
				IsTerminated:       false,
			}
			if err := tx.Create(&newRow).Error; err != nil {
				var mysqlErr *mysql.MySQLError
				if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
						Where("session_id = ?", sessionID).
						First(&existing).Error; err != nil {
						return err
					}
					if existing.UserID == nil {
						existing.UserID = &userID
						return tx.Model(&existing).Update("user_id", userID).Error
					}
					if *existing.UserID != userID {
						return shared.ErrForbidden
					}
					return nil
				}
				return err
			}
			return nil
		}

		if existing.UserID == nil {
			existing.UserID = &userID
			return tx.Model(&existing).Update("user_id", userID).Error
		}
		if *existing.UserID != userID {
			return shared.ErrForbidden
		}
		return nil
	})
}

// GetOrCreate セッションのバリデーション情報を取得または作成
func (r *SessionValidationRepository) GetOrCreate(sessionID string) (*models.SessionValidation, error) {
	var validation models.SessionValidation
	err := r.db.Where("session_id = ?", sessionID).First(&validation).Error
	if err == gorm.ErrRecordNotFound {
		validation = models.SessionValidation{
			SessionID:          sessionID,
			InvalidAnswerCount: 0,
			IsTerminated:       false,
		}
		if err := r.db.Create(&validation).Error; err != nil {
			return nil, err
		}
		return &validation, nil
	}
	if err != nil {
		return nil, err
	}
	return &validation, nil
}

// IncrementInvalidCount 無効回答カウントをインクリメント
func (r *SessionValidationRepository) IncrementInvalidCount(sessionID string) (*models.SessionValidation, error) {
	validation, err := r.GetOrCreate(sessionID)
	if err != nil {
		return nil, err
	}

	validation.InvalidAnswerCount++
	now := time.Now()
	validation.LastInvalidAnswerTime = &now

	if err := r.db.Save(validation).Error; err != nil {
		return nil, err
	}

	return validation, nil
}

// ResetInvalidCount 無効回答カウントをリセット
func (r *SessionValidationRepository) ResetInvalidCount(sessionID string) error {
	validation, err := r.GetOrCreate(sessionID)
	if err != nil {
		return err
	}

	validation.InvalidAnswerCount = 0
	return r.db.Save(validation).Error
}

// TerminateSession セッションを強制終了
func (r *SessionValidationRepository) TerminateSession(sessionID string) error {
	validation, err := r.GetOrCreate(sessionID)
	if err != nil {
		return err
	}

	validation.IsTerminated = true
	return r.db.Save(validation).Error
}

// IsTerminated セッションが終了しているかチェック
func (r *SessionValidationRepository) IsTerminated(sessionID string) (bool, error) {
	validation, err := r.GetOrCreate(sessionID)
	if err != nil {
		return false, err
	}
	return validation.IsTerminated, nil
}
