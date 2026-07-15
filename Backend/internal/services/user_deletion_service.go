package services

import (
	"Backend/internal/models"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/gorm"
)

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

// UserDeletionService は退会・管理者削除のカスケード（DB + S3）を担当する。
type UserDeletionService struct {
	db     *gorm.DB
	object ObjectDeleter // nil 可（ローカル等）
	audit  auditRecorder
}

func NewUserDeletionService(db *gorm.DB, object ObjectDeleter, audit auditRecorder) *UserDeletionService {
	return &UserDeletionService{db: db, object: object, audit: audit}
}

// DeleteUser はユーザー個人データと関連オブジェクトを削除する。
func (s *UserDeletionService) DeleteUser(userID uint, actor UserDeletionActor) error {
	if s.db == nil {
		return errors.New("database not configured")
	}

	var (
		email   string
		s3Keys  []string
		s3Errs  []string
	)

	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}
		email = user.Email

		keys, err := collectUserObjectKeys(tx, userID)
		if err != nil {
			return err
		}
		s3Keys = keys

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

		if err := tx.Where("user_id = ?", userID).Delete(&models.ChatMessage{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.AIGeneratedQuestion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ConversationContext{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserWeightScore{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserAnalysisProgress{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserEmbedding{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.VariantAssignment{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserCompanyMatch{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserApplicationStatus{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.CompanyReview{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ResumeDocument{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.InterviewSession{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.InterviewVideo{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.RealtimeUsageLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.ScheduleEvent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.SkillScore{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubRepoSummary{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubLanguageStat{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubRepo{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.GitHubProfile{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserGoogleToken{}).Error; err != nil {
			return err
		}
		// 集合知ログは匿名ハッシュのみだが、退会時に同一ハッシュを削除して再紐付けリスクを下げる
		if err := tx.Where("anonymous_user_id = ?", collectiveAnonymousHash(userID)).Delete(&models.CollectiveInsightLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("email = ?", user.Email).Delete(&models.PendingRegistration{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.User{}, userID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	s3Errs = s.deleteObjectsBestEffort(s3Keys)

	action := "user.self_delete"
	actorEmail := email
	if actor.Kind == "admin" {
		action = "user.admin_delete"
		if actor.Email != "" {
			actorEmail = actor.Email
		}
	}
	if s.audit != nil {
		meta := map[string]any{
			"deleted_user_id":    userID,
			"deleted_user_email": email,
			"s3_object_count":    len(s3Keys),
		}
		if len(s3Errs) > 0 {
			meta["s3_errors"] = s3Errs
		}
		s.audit.Record(actorEmail, action, "user", userID, meta)
	}
	return nil
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
		// S3 未設定時はローカルパスを best-effort で削除
		if strings.HasPrefix(key, "/") || strings.Contains(key, string(os.PathSeparator)) {
			if err := os.Remove(key); err != nil && !os.IsNotExist(err) {
				log.Printf("[UserDeletion] local file delete failed path=%s: %v", key, err)
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
		if bucket, key, ok := parseS3URI(raw); ok {
			_ = bucket
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
