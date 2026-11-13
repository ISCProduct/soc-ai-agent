package auth

import "errors"

// DeleteAccount ユーザーアカウントとその全データを削除する（個人情報保護法第28条対応）
func (s *AuthService) DeleteAccount(userID uint) error {
	if s.deletion == nil {
		return errors.New("database not configured")
	}
	// リフレッシュトークンの失効は DeleteUser のトランザクション内で行われる (#616)
	return s.deletion.DeleteUser(userID, UserDeletionActor{Kind: "self"})
}
