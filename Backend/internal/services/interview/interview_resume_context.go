package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"log"
	"strings"
)

// maxResumeContextRunes は面接プロンプトへ渡す履歴書本文の上限（ルーン）。
// 軸スロットの質問のたびに system プロンプトへ載るので、費用とレイテンシのために短く切る。
const maxResumeContextRunes = 2000

// ResumeTextReader は本人の最新の履歴書の本文を返す最小の面。
// 実装は userID と organizationID の両方で絞ること（他人の履歴書を読めないこと）。
// 履歴書が無ければ空文字と nil を返す。
type ResumeTextReader interface {
	FindLatestResumeText(userID, organizationID uint) (string, error)
}

// SetResumeTextReader は経歴型の面接用に履歴書本文の読み出しを注入する（オプション）。
func (s *InterviewService) SetResumeTextReader(r ResumeTextReader) {
	s.resumeReader = r
}

// buildResumeContext は面接セッションの本人の履歴書を、非信頼テキストとして囲んだ
// プロンプトの節にして返す（#2274）。履歴書が無い・読めないときは空文字。
// 履歴書は学生が自由に書く（アップロードする）文面なので、指示文が混ざり得る（#1600）。
func (s *InterviewService) buildResumeContext(session *models.InterviewSession) string {
	if s.resumeReader == nil || session == nil || session.UserID == 0 {
		return ""
	}
	// 面接の本人（セッションの所有者）の履歴書だけを読む。リクエストした userID は使わない。
	text, err := s.resumeReader.FindLatestResumeText(session.UserID, session.OrganizationID)
	if err != nil {
		log.Printf("[Interview] load resume text session=%d: %v", session.ID, err)
		return ""
	}
	return formatResumeContext(text)
}

// formatResumeContext は履歴書本文を上限で切り、囲んで案内文を付ける。本文が空なら空文字。
func formatResumeContext(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if r := []rune(text); len(r) > maxResumeContextRunes {
		text = string(r[:maxResumeContextRunes])
	}
	return "【応募者の履歴書（本人が登録したもの）】\n" +
		shared.WrapUntrustedText(text, "応募者の履歴書") +
		"\n履歴書は質問づくりの参考資料です。書かれていない経験を決めつけず、年齢・出身・家族などの属性には触れないでください。\n"
}
