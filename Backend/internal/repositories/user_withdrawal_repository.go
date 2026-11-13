package repositories

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// UserWithdrawalRepository は repository.UserWithdrawalRepository の GORM 実装。
type UserWithdrawalRepository struct {
	db *gorm.DB
}

func NewUserWithdrawalRepository(db *gorm.DB) *UserWithdrawalRepository {
	return &UserWithdrawalRepository{db: db}
}

func (r *UserWithdrawalRepository) Withdraw(userID uint, now time.Time, build repository.WithdrawalRecordBuilder) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		if user.IsWithdrawn() {
			return repository.ErrAlreadyWithdrawn
		}
		email := user.Email

		keys, err := collectUserObjectKeys(tx, userID)
		if err != nil {
			return err
		}

		record := build(&user, keys)
		if err := tx.Create(&record).Error; err != nil {
			return err
		}

		// 元のメールアドレスの仮登録（本登録が完了する前に作られた行）を消す。
		// 下でメールをスクランブルすると、パージ時には元のアドレスが分からず消せなくなる（#2140 のレビュー）。
		// 消せなければ退会全体を巻き戻す（個人情報を残したまま退会済みにしない）。0件は正常。
		if err := tx.Where("email = ?", email).Delete(&models.PendingRegistration{}).Error; err != nil {
			return fmt.Errorf("delete pending registrations: %w", err)
		}

		// メール再利用を許し、ログイン不能にする
		user.WithdrawnAt = &now
		user.Email = fmt.Sprintf("withdrawn+%d@deleted.invalid", userID)
		user.Password = ""
		user.OAuthProvider = ""
		user.OAuthID = ""
		user.EmailVerificationToken = ""
		user.PasswordResetToken = ""
		user.IsAdmin = false
		if err := tx.Save(&user).Error; err != nil {
			return err
		}

		// 全端末のリフレッシュトークンを即時失効させる（self/admin 両経路 #616）
		return tx.Model(&models.UserRefreshToken{}).
			Where("user_id = ? AND revoked_at IS NULL", userID).
			Update("revoked_at", now).Error
	})
}

func (r *UserWithdrawalRepository) HardDeleteUser(userID uint, collectiveAnonymousID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return hardDeleteUserData(tx, userID, collectiveAnonymousID)
	})
}

func (r *UserWithdrawalRepository) ListDueWithdrawals(now time.Time) ([]models.WithdrawnUser, error) {
	var due []models.WithdrawnUser
	if err := r.db.Where("purged_at IS NULL AND purge_after <= ?", now).Find(&due).Error; err != nil {
		return nil, err
	}
	return due, nil
}

func (r *UserWithdrawalRepository) ListPendingWithdrawals() ([]models.WithdrawnUser, error) {
	var pending []models.WithdrawnUser
	if err := r.db.Where("purged_at IS NULL").Find(&pending).Error; err != nil {
		return nil, err
	}
	return pending, nil
}

func (r *UserWithdrawalRepository) MarkPurged(withdrawnUserID uint, at time.Time) error {
	return r.db.Model(&models.WithdrawnUser{}).Where("id = ?", withdrawnUserID).Updates(map[string]any{
		"purged_at": at,
	}).Error
}

func (r *UserWithdrawalRepository) FindWithdrawalState(userID uint) (*models.User, bool, error) {
	var user models.User
	if err := r.db.Select("id", "withdrawn_at").First(&user, userID).Error; err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &user, true, nil
}

func hardDeleteUserData(tx *gorm.DB, userID uint, collectiveAnonymousID string) error {
	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}

	// 個人の AI 利用上限。scope_id は外部キーを持てないので明示的に消す。
	if err := tx.Where("scope = ? AND scope_id = ?", models.AIBudgetScopeUser, userID).Delete(&models.AIBudgetSetting{}).Error; err != nil {
		return err
	}

	// リフレッシュトークンを削除（#616）
	if err := tx.Where("user_id = ?", userID).Delete(&models.UserRefreshToken{}).Error; err != nil {
		return err
	}

	sessionIDs, err := collectUserSessionIDs(tx, userID)
	if err != nil {
		return err
	}
	if len(sessionIDs) > 0 {
		if err := tx.Where("session_id IN ?", sessionIDs).Delete(&models.SessionValidation{}).Error; err != nil {
			return err
		}
	}

	interviewSessionIDs, err := collectInterviewSessionIDs(tx, userID)
	if err != nil {
		return err
	}
	if len(interviewSessionIDs) > 0 {
		if err := tx.Where("interview_session_id IN ?", interviewSessionIDs).Delete(&models.RealtimeUsageLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id IN ?", interviewSessionIDs).Delete(&models.InterviewUtterance{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id IN ?", interviewSessionIDs).Delete(&models.InterviewReport{}).Error; err != nil {
			return err
		}
		if err := tx.Where("session_id IN ?", interviewSessionIDs).Delete(&models.InterviewQuestionState{}).Error; err != nil {
			return err
		}
	}

	resumeDocumentIDs, err := collectResumeDocumentIDs(tx, userID)
	if err != nil {
		return err
	}
	if len(resumeDocumentIDs) > 0 {
		resumeReviewIDs, err := collectResumeReviewIDs(tx, resumeDocumentIDs)
		if err != nil {
			return err
		}
		if len(resumeReviewIDs) > 0 {
			if err := tx.Where("review_id IN ?", resumeReviewIDs).Delete(&models.ResumeReviewItem{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("document_id IN ?", resumeDocumentIDs).Delete(&models.ResumeTextBlock{}).Error; err != nil {
			return err
		}
		if err := tx.Where("document_id IN ?", resumeDocumentIDs).Delete(&models.ResumeReview{}).Error; err != nil {
			return err
		}
	}

	deletes := []any{
		&models.ChatMessage{},
		&models.AIGeneratedQuestion{},
		&models.ConversationContext{},
		&models.UserWeightScore{},
		// 再計算の控え（#1770）。scores_json にスコアを丸ごと持つので、退会後に残さない
		&models.UserScoreRecalculationSnapshot{},
		&models.UserAnalysisProgress{},
		&models.UserEmbedding{},
		&models.VariantAssignment{},
		&models.UserCompanyMatch{},
		&models.UserApplicationStatus{},
		&models.CompanyReview{},
		&models.ResumeDocument{},
		&models.InterviewSession{},
		&models.InterviewVideo{},
		&models.RealtimeUsageLog{},
		&models.ScheduleEvent{},
		&models.SkillScore{},
		&models.GitHubRepoSummary{},
		&models.GitHubLanguageStat{},
		&models.GitHubRepo{},
		&models.GitHubProfile{},
		&models.UserGoogleToken{},
		// 適性検査（#1678）。回答は assessment_sessions の FK CASCADE で消える
		&models.AssessmentResult{},
		&models.AssessmentSession{},
		// ES添削（#1531）。ES本文・改善文を持つので明示的に消す。
		// users への FK CASCADE でも落ちるが、依存を暗黙にしない。
		&models.ESReview{},
		// アプリ内通知（#2033）。先生の案内文や企業名を含むので、退会後に残さない（users への FK は無い）。
		&models.Notification{},
	}
	for _, m := range deletes {
		if err := tx.Where("user_id = ?", userID).Delete(m).Error; err != nil {
			return err
		}
	}
	// 教員案内は生徒・送信教員のどちらとして退会しても本文が残らないよう両側で消す
	if err := tx.Where("student_user_id = ? OR teacher_user_id = ?", userID, userID).Delete(&models.TeacherStudentGuidance{}).Error; err != nil {
		return err
	}
	// organization_memberships は users への FK があるため、ユーザー削除前に除去する
	if err := tx.Where("user_id = ?", userID).Delete(&models.OrganizationMembership{}).Error; err != nil {
		return err
	}
	if err := tx.Where("anonymous_user_id = ?", collectiveAnonymousID).Delete(&models.CollectiveInsightLog{}).Error; err != nil {
		return err
	}
	// 退会時にスクランブル済みメールだが、念のため元パターンでも消す。
	// 仮登録にはメールアドレスが入っているので、消せなければ全体を巻き戻し、次回のパージで再試行する。
	// 握りつぶすとコミットされ、物理削除したのに仮登録だけが残る（#2001）。0件は正常（Delete はエラーにならない）。
	if err := tx.Where("email = ? OR email LIKE ?", user.Email, fmt.Sprintf("withdrawn+%d@%%", userID)).Delete(&models.PendingRegistration{}).Error; err != nil {
		return fmt.Errorf("delete pending registrations: %w", err)
	}
	return tx.Delete(&models.User{}, userID).Error
}

// collectUserObjectKeys はユーザーが持つオブジェクト(面接動画・履歴書)の保存先を、
// 保存されているままの文字列で返す。S3 の URI からキーへの正規化と重複排除は、DB の外の手順。
func collectUserObjectKeys(tx *gorm.DB, userID uint) ([]string, error) {
	var out []string

	var videos []models.InterviewVideo
	if err := tx.Where("user_id = ?", userID).Find(&videos).Error; err != nil {
		return nil, err
	}
	for _, v := range videos {
		out = append(out, v.DriveFileID)
	}

	var docs []models.ResumeDocument
	if err := tx.Where("user_id = ?", userID).Find(&docs).Error; err != nil {
		return nil, err
	}
	for _, d := range docs {
		out = append(out, d.StoredPath, d.NormalizedPath, d.AnnotatedPath)
	}
	return out, nil
}

func collectUserSessionIDs(tx *gorm.DB, userID uint) ([]string, error) {
	sessions := map[string]struct{}{}
	collect := func(model any) error {
		var ids []string
		if err := tx.Model(model).
			Where("user_id = ?", userID).
			Distinct().
			Pluck("session_id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if id != "" {
				sessions[id] = struct{}{}
			}
		}
		return nil
	}

	if err := collect(&models.ChatMessage{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserWeightScore{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ConversationContext{}); err != nil {
		return nil, err
	}
	if err := collect(&models.AIGeneratedQuestion{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserAnalysisProgress{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserEmbedding{}); err != nil {
		return nil, err
	}
	if err := collect(&models.UserCompanyMatch{}); err != nil {
		return nil, err
	}
	if err := collect(&models.VariantAssignment{}); err != nil {
		return nil, err
	}
	if err := collect(&models.ResumeDocument{}); err != nil {
		return nil, err
	}

	result := make([]string, 0, len(sessions))
	for id := range sessions {
		result = append(result, id)
	}
	return result, nil
}

func collectInterviewSessionIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var sessionIDs []uint
	if err := tx.Model(&models.InterviewSession{}).
		Where("user_id = ?", userID).
		Pluck("id", &sessionIDs).Error; err != nil {
		return nil, err
	}
	return sessionIDs, nil
}

func collectResumeDocumentIDs(tx *gorm.DB, userID uint) ([]uint, error) {
	var documentIDs []uint
	if err := tx.Model(&models.ResumeDocument{}).
		Where("user_id = ?", userID).
		Pluck("id", &documentIDs).Error; err != nil {
		return nil, err
	}
	return documentIDs, nil
}

func collectResumeReviewIDs(tx *gorm.DB, documentIDs []uint) ([]uint, error) {
	var reviewIDs []uint
	if err := tx.Model(&models.ResumeReview{}).
		Where("document_id IN ?", documentIDs).
		Pluck("id", &reviewIDs).Error; err != nil {
		return nil, err
	}
	return reviewIDs, nil
}
