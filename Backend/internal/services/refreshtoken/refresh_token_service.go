package refreshtoken

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"Backend/internal/models"
)

const (
	// refreshTokenTTL はリフレッシュトークンの有効期間
	refreshTokenTTL = 30 * 24 * time.Hour
	// rotationGracePeriod はローテーション直後の旧トークンを許容する猶予。
	// 並行リクエストが同時にリフレッシュした場合のログアウト事故を防ぐ。
	rotationGracePeriod = 60 * time.Second
)

// ErrInvalidRefreshToken は無効（不明・失効・期限切れ）なリフレッシュトークンを表す
var ErrInvalidRefreshToken = errors.New("invalid refresh token")

// RefreshTokenStore は RefreshTokenService が必要とする永続化操作
type RefreshTokenStore interface {
	Create(token *models.UserRefreshToken) error
	FindByHash(hash string) (*models.UserRefreshToken, error)
	Revoke(id uint, at time.Time) error
	RevokeAllByUser(userID uint, at time.Time) error
}

// RefreshTokenService はリフレッシュトークンの発行・ローテーション・失効を担当する (#616)
type RefreshTokenService struct {
	store RefreshTokenStore
	now   func() time.Time // テスト用に差し替え可能
}

func NewRefreshTokenService(store RefreshTokenStore) *RefreshTokenService {
	return &RefreshTokenService{store: store, now: time.Now}
}

// hashRefreshToken はトークン平文の SHA-256 ハッシュ（hex）を返す
func hashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// Issue は新しいリフレッシュトークンを発行し、平文を返す（保存はハッシュのみ）
func (s *RefreshTokenService) Issue(userID uint) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("リフレッシュトークン生成に失敗: %w", err)
	}
	plain := hex.EncodeToString(buf)

	token := &models.UserRefreshToken{
		UserID:    userID,
		TokenHash: hashRefreshToken(plain),
		ExpiresAt: s.now().Add(refreshTokenTTL),
	}
	if err := s.store.Create(token); err != nil {
		return "", fmt.Errorf("リフレッシュトークン保存に失敗: %w", err)
	}
	return plain, nil
}

// Rotate はトークンを検証し、旧トークンを失効させて新しいトークンを発行する。
// 戻り値はユーザーIDと新トークンの平文。無効な場合は ErrInvalidRefreshToken。
func (s *RefreshTokenService) Rotate(plain string) (uint, string, error) {
	token, err := s.validate(plain)
	if err != nil {
		return 0, "", err
	}

	// 旧トークンを失効（猶予期間内の再利用は validate 側で許容される）
	if token.RevokedAt == nil {
		if err := s.store.Revoke(token.ID, s.now()); err != nil {
			return 0, "", fmt.Errorf("旧トークンの失効に失敗: %w", err)
		}
	}

	newPlain, err := s.Issue(token.UserID)
	if err != nil {
		return 0, "", err
	}
	return token.UserID, newPlain, nil
}

// Revoke はトークンを失効させる（見つからない場合もエラーにしない）
func (s *RefreshTokenService) Revoke(plain string) error {
	token, err := s.store.FindByHash(hashRefreshToken(plain))
	if err != nil {
		return err
	}
	if token == nil || token.RevokedAt != nil {
		return nil
	}
	return s.store.Revoke(token.ID, s.now())
}

// RevokeAllForUser はユーザーの全トークンを失効させる
// （パスワード変更・アカウント削除・全端末ログアウト時に使用）
func (s *RefreshTokenService) RevokeAllForUser(userID uint) error {
	return s.store.RevokeAllByUser(userID, s.now())
}

// validate はトークンの存在・期限・失効状態を検証する
func (s *RefreshTokenService) validate(plain string) (*models.UserRefreshToken, error) {
	if plain == "" {
		return nil, ErrInvalidRefreshToken
	}
	token, err := s.store.FindByHash(hashRefreshToken(plain))
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, ErrInvalidRefreshToken
	}
	now := s.now()
	if now.After(token.ExpiresAt) {
		return nil, ErrInvalidRefreshToken
	}
	// 失効済みでも猶予期間内なら許容（並行リフレッシュ対策）
	if token.RevokedAt != nil && now.Sub(*token.RevokedAt) > rotationGracePeriod {
		return nil, ErrInvalidRefreshToken
	}
	return token, nil
}
