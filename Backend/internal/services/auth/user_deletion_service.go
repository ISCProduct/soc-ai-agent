package auth

import (
	"Backend/domain/repository"
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
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
	repo             repository.UserWithdrawalRepository
	object           ObjectDeleter
	audit            auditRecorder
	scoutIndexSyncer ScoutIndexSyncer
}

func NewUserDeletionService(repo repository.UserWithdrawalRepository, object ObjectDeleter, audit auditRecorder) *UserDeletionService {
	return &UserDeletionService{repo: repo, object: object, audit: audit}
}

// SetScoutIndexSyncer は退会時にスカウト検索インデックスを同期する処理を注入する（#1094）。
// 未設定なら同期しない（RAG未設定環境でも退会は成功する）。
func (s *UserDeletionService) SetScoutIndexSyncer(syncer ScoutIndexSyncer) {
	s.scoutIndexSyncer = syncer
}

var (
	// ErrAlreadyWithdrawn は repository.ErrAlreadyWithdrawn と同じ値。errors.Is / == どちらでも判定できる。
	ErrAlreadyWithdrawn = repository.ErrAlreadyWithdrawn
	ErrAccountWithdrawn = errors.New("account has been withdrawn")
)

// UserAccessGuard は退会済みユーザーの API 利用を遮断するためのインターフェース。
type UserAccessGuard interface {
	EnsureActiveUser(userID uint) error
}

// DeleteUser は退会処理（論理削除）。S3・関連行は残し、閲覧・ログインを不可にする。
func (s *UserDeletionService) DeleteUser(userID uint, actor UserDeletionActor) error {
	if s.repo == nil {
		return errors.New("database not configured")
	}

	now := time.Now().UTC()
	purgeAfter := now.AddDate(0, 0, WithdrawalRetentionDays)
	var email string
	var keyCount int

	err := s.repo.Withdraw(userID, now, func(user *models.User, rawObjectKeys []string) models.WithdrawnUser {
		email = user.Email
		keys := normalizeObjectKeys(rawObjectKeys)
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

		return models.WithdrawnUser{
			UserID:       userID,
			EmailHash:    hashEmail(email),
			EmailMasked:  maskEmail(email),
			Reason:       reason,
			ActorEmail:   actorEmail,
			S3ObjectKeys: string(keysJSON),
			WithdrawnAt:  now,
			PurgeAfter:   purgeAfter,
		}
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

	// 退会後もスカウト検索のベクトルが残り続けないよう削除する（#1094）。
	// 退会済みは IsScoutVisible が false を返すため、Sync が削除を行う。
	// RAG障害で退会自体を失敗させたくないので、Sync 側でエラーはログに留める。
	if s.scoutIndexSyncer != nil {
		s.scoutIndexSyncer.Sync(context.Background(), userID)
	}
	return nil
}

// IsUserWithdrawn はユーザーが退会済み（未パージ）かどうかを返す。
func (s *UserDeletionService) IsUserWithdrawn(userID uint) (bool, error) {
	if s.repo == nil {
		return false, errors.New("database not configured")
	}
	user, found, err := s.repo.FindWithdrawalState(userID)
	if err != nil {
		return false, err
	}
	if !found {
		return true, nil // 完全削除済みも閲覧不可
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
	if s.repo == nil {
		return 0, errors.New("database not configured")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	due, err := s.repo.ListDueWithdrawals(now)
	if err != nil {
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

// scoutIndexDeleter は退会済みユーザーのベクトルを確実に消すための最小インターフェース。
// ScoutIndexSyncer.Sync は成否を返さないため、リコンサイルではこちらを使う（#1204）。
type scoutIndexDeleter interface {
	EnsureDeleted(ctx context.Context, userID uint) error
}

// ReconcileScoutIndex は退会済み（未パージ）ユーザーのスカウト用ベクトルを消し直す（#1204）。
//
// 退会時の削除は RAG 障害で退会自体を失敗させないためエラーを握りつぶしており、
// ChromaDB が止まっている間に退会したユーザーのベクトルは残ったままになる。
// 検索結果は DB 側で再フィルタされるため情報漏洩は起きないが、
// 個人データが消えない状態が誰にも気づかれずに続く。
//
// 物理パージ(30日)を待つと窓が広すぎるので、パージと同じ日次ジョブで消し直す。
// 削除は冪等なので、既に消えているユーザーに対して繰り返し呼んでも副作用はない。
//
// 戻り値: (試行件数, 失敗件数, エラー)。エラーは対象の取得に失敗した場合のみ。
func (s *UserDeletionService) ReconcileScoutIndex(ctx context.Context) (int, int, error) {
	if s.repo == nil {
		return 0, 0, errors.New("database not configured")
	}
	deleter, ok := s.scoutIndexSyncer.(scoutIndexDeleter)
	if !ok || deleter == nil {
		// 同期先が未設定の構成（テストやRAG無効時）では何もしない
		return 0, 0, nil
	}

	pending, err := s.repo.ListPendingWithdrawals()
	if err != nil {
		return 0, 0, err
	}

	failed := 0
	for _, w := range pending {
		if err := deleter.EnsureDeleted(ctx, w.UserID); err != nil {
			// 個々の失敗で全体を止めない。次回の実行で再試行される。
			log.Printf("[UserDeletion] scout index reconcile failed user_id=%d: %v", w.UserID, err)
			failed++
		}
	}
	return len(pending), failed, nil
}

func (s *UserDeletionService) purgeOne(w models.WithdrawnUser) error {
	var keys []string
	_ = json.Unmarshal([]byte(w.S3ObjectKeys), &keys)

	if err := s.repo.HardDeleteUser(w.UserID, collectiveAnonymousHash(w.UserID)); err != nil {
		return err
	}

	s3Errs := s.deleteObjectsBestEffort(keys)
	now := time.Now().UTC()
	// 物理削除はもうコミット済みなので、記録に失敗しても成功として扱う（エラーを返すと「削除失敗」と記録され実態とずれる）。
	// purged_at が残らないと次回のパージでもう一度対象になるが、消えたデータを消し直すだけなので害は無い。
	// 失敗はログで分かるようにする（#2001）。
	if err := s.repo.MarkPurged(w.ID, now); err != nil {
		log.Printf("[UserDeletion] purged but failed to record purged_at withdrawn_user_id=%d user_id=%d: %v", w.ID, w.UserID, err)
	}

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

// normalizeObjectKeys は、保存されている保存先の文字列から、S3 のキー(または手元のパス)を取り出す。
// S3 の URI はバケットを除いたキーにし、空と重複は除く。並びは安定させる(昇順)。
func normalizeObjectKeys(raw []string) []string {
	seen := map[string]struct{}{}
	for _, r := range raw {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, key, ok := shared.ParseS3URI(r); ok {
			r = key
		}
		if r == "" {
			continue
		}
		seen[r] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
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

// MaskEmail はログ向けにローカル部を伏せる。登録前でユーザーIDが無いときに使う。
func MaskEmail(email string) string {
	return maskEmail(email)
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
