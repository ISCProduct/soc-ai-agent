package services

import (
	"Backend/internal/models"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
)

const WithdrawalRetentionDays = 30

// ObjectDeleter は S3 等のオブジェクト削除インターフェース。
type ObjectDeleter interface {
	DeleteObject(ctx context.Context, key string) error
}

// auditRecorder は循環 import を避けるための最小監査インターフェース。
type auditRecorder interface {
	Record(actorEmail, action, targetType string, targetID uint, metadata map[string]any)
}

// UserDeletionActor は削除実行者（本人 / 管理者）。
type UserDeletionActor struct {
	Kind  string // self | admin
	Email string
}

// UserDeletionService は退会（論理）と猶予後の物理パージを担当する。
type UserDeletionService struct {
	db     *gorm.DB
	object ObjectDeleter
	audit  auditRecorder
}

func NewUserDeletionService(db *gorm.DB, object ObjectDeleter, audit auditRecorder) *UserDeletionService {
	return &UserDeletionService{db: db, object: object, audit: audit}
}

var (
	ErrAlreadyWithdrawn = errors.New("account already withdrawn")
	ErrAccountWithdrawn = errors.New("account has been withdrawn")
)

// UserAccessGuard は退会済みユーザーの API 利用を遮断するためのインターフェース。
type UserAccessGuard interface {
	EnsureActiveUser(userID uint) error
}

// DeleteUser は退会処理（論理削除）。S3・関連行は残し、閲覧・ログインを不可にする。
func (s *UserDeletionService) DeleteUser(userID uint, actor UserDeletionActor) error {
	if s.db == nil {
		return errors.New("database not configured")
	}

	now := time.Now().UTC()
	purgeAfter := now.AddDate(0, 0, WithdrawalRetentionDays)
	var email string
	var keyCount int

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		if user.IsWithdrawn() {
			return ErrAlreadyWithdrawn
		}
		email = user.Email

		keys, err := collectUserObjectKeys(tx, userID)
		if err != nil {
			return err
		}
		keyCount = len(keys)
		keysJSON, _ := json.Marshal(keys)

		reason := "self"
		actorEmail := email
		if actor.Kind == "admin" {
			reason = "admin"
			if actor.Email != "" {
				actorEmail = actor.Email
			}
		}

		record := models.WithdrawnUser{
			UserID:       userID,
			EmailHash:    hashEmail(email),
			EmailMasked:  maskEmail(email),
			Reason:       reason,
			ActorEmail:   actorEmail,
			S3ObjectKeys: string(keysJSON),
			WithdrawnAt:  now,
			PurgeAfter:   purgeAfter,
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
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
		return tx.Save(&user).Error
	})
	if err != nil {
		return err
	}

	action := "user.self_withdraw"
	actorEmail := email
	if actor.Kind == "admin" {
		action = "user.admin_withdraw"
		if actor.Email != "" {
			actorEmail = actor.Email
		}
	}
	if s.audit != nil {
		s.audit.Record(actorEmail, action, "user", userID, map[string]any{
			"deleted_user_email_masked": maskEmail(email),
			"purge_after":               purgeAfter.Format(time.RFC3339),
			"s3_object_count":           keyCount,
			"retention_days":            WithdrawalRetentionDays,
		})
	}
	return nil
}

// IsUserWithdrawn はユーザーが退会済み（未パージ）かどうかを返す。
func (s *UserDeletionService) IsUserWithdrawn(userID uint) (bool, error) {
	if s.db == nil {
		return false, errors.New("database not configured")
	}
	var user models.User
	if err := s.db.Select("id", "withdrawn_at").First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return true, nil // 完全削除済みも閲覧不可
		}
		return false, err
	}
	return user.IsWithdrawn(), nil
}

// EnsureActiveUser はアクティブユーザーでなければエラーを返す（認証ミドルウェア用）。
func (s *UserDeletionService) EnsureActiveUser(userID uint) error {
	withdrawn, err := s.IsUserWithdrawn(userID)
	if err != nil {
		return err
	}
	if withdrawn {
		return ErrAccountWithdrawn
	}
	return nil
}

// PurgeExpiredWithdrawals は PurgeAfter を過ぎた退会ユーザーを物理削除する。
func (s *UserDeletionService) PurgeExpiredWithdrawals(now time.Time) (int, error) {
	if s.db == nil {
		return 0, errors.New("database not configured")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var due []models.WithdrawnUser
	if err := s.db.Where("purged_at IS NULL AND purge_after <= ?", now).Find(&due).Error; err != nil {
		return 0, err
	}

	purged := 0
	for _, w := range due {
		if err := s.purgeOne(w); err != nil {
			log.Printf("[UserDeletion] purge failed user_id=%d: %v", w.UserID, err)
			continue
		}
		purged++
	}
	return purged, nil
}

func (s *UserDeletionService) purgeOne(w models.WithdrawnUser) error {
	var keys []string
	_ = json.Unmarshal([]byte(w.S3ObjectKeys), &keys)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		return hardDeleteUserData(tx, w.UserID)
	})
	if err != nil {
		return err
	}

	s3Errs := s.deleteObjectsBestEffort(keys)
	now := time.Now().UTC()
	_ = s.db.Model(&models.WithdrawnUser{}).Where("id = ?", w.ID).Updates(map[string]any{
		"purged_at": now,
	}).Error

	if s.audit != nil {
		meta := map[string]any{
			"withdrawn_user_id": w.ID,
			"purge_after":       w.PurgeAfter.Format(time.RFC3339),
			"s3_object_count":   len(keys),
		}
		if len(s3Errs) > 0 {
			meta["s3_errors"] = s3Errs
		}
		s.audit.Record("system", "user.purge", "user", w.UserID, meta)
	}
	return nil
}

func hardDeleteUserData(tx *gorm.DB, userID uint) error {
	var user models.User
	if err := tx.First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
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
	}
	for _, m := range deletes {
		if err := tx.Where("user_id = ?", userID).Delete(m).Error; err != nil {
			return err
		}
	}
	if err := tx.Where("anonymous_user_id = ?", collectiveAnonymousHash(userID)).Delete(&models.CollectiveInsightLog{}).Error; err != nil {
		return err
	}
	// 退会時にスクランブル済みメールだが、念のため元パターンでも消す
	_ = tx.Where("email = ? OR email LIKE ?", user.Email, fmt.Sprintf("withdrawn+%d@%%", userID)).Delete(&models.PendingRegistration{})
	return tx.Delete(&models.User{}, userID).Error
}

func (s *UserDeletionService) deleteObjectsBestEffort(keys []string) []string {
	if len(keys) == 0 {
		return nil
	}
	var errs []string
	ctx := context.Background()
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if s.object != nil {
			if err := s.object.DeleteObject(ctx, key); err != nil {
				log.Printf("[UserDeletion] s3 delete failed key=%s: %v", key, err)
				errs = append(errs, fmt.Sprintf("%s: %v", key, err))
			}
			continue
		}
		if strings.HasPrefix(key, "/") || strings.Contains(key, string(os.PathSeparator)) {
			if err := os.Remove(key); err != nil && !os.IsNotExist(err) {
				errs = append(errs, fmt.Sprintf("%s: %v", key, err))
			}
		}
	}
	return errs
}

func collectUserObjectKeys(tx *gorm.DB, userID uint) ([]string, error) {
	seen := map[string]struct{}{}
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, key, ok := parseS3URI(raw); ok {
			raw = key
		}
		if raw == "" {
			return
		}
		seen[raw] = struct{}{}
	}

	var videos []models.InterviewVideo
	if err := tx.Where("user_id = ?", userID).Find(&videos).Error; err != nil {
		return nil, err
	}
	for _, v := range videos {
		add(v.DriveFileID)
	}

	var docs []models.ResumeDocument
	if err := tx.Where("user_id = ?", userID).Find(&docs).Error; err != nil {
		return nil, err
	}
	for _, d := range docs {
		add(d.StoredPath)
		add(d.NormalizedPath)
		add(d.AnnotatedPath)
	}

	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out, nil
}

func collectiveAnonymousHash(userID uint) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("user:%d:collective", userID)))
	return hex.EncodeToString(h[:])
}

func hashEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	if at <= 0 {
		return "***"
	}
	local := email[:at]
	domain := email[at:]
	if len(local) == 1 {
		return local + "***" + domain
	}
	return string(local[0]) + "***" + domain
}
