package chat

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// 相談の意図（ホームの「キャリア相談」の4つの選択肢と、自由記述）。SOCAIAGENT-668。
// フロント（frontend/lib/career-consult.ts）と値を合わせる。
const (
	ConsultIntentFindJob           = "find_job"           // やりたい仕事を一緒に考えたい
	ConsultIntentLearnIndustry     = "learn_industry"     // 志望する業界について知りたい
	ConsultIntentHowToProceed      = "how_to_proceed"     // 就活の進め方がわからない
	ConsultIntentOrganizeStrengths = "organize_strengths" // 自分の強みを整理したい
	ConsultIntentFree              = "free"               // 自分の言葉で入力した
)

// MaxChatMessageRunes はチャット本文の文字数（ルーン）の上限（SOCAIAGENT-702）。
// 仮の値。実際の長さの分布を見て決め直す。フロントの入力欄（CHAT_MESSAGE_MAX_CHARS）と同じ値にすること。
const MaxChatMessageRunes = 2000

// ErrChatMessageTooLong はチャット本文が MaxChatMessageRunes を超えたことを表す。文言はそのまま利用者に見える。
var ErrChatMessageTooLong = errors.New("文字数が上限を超えています")

// ValidateChatMessageLength はチャット本文が上限以内かを確かめる。
func ValidateChatMessageLength(message string) error {
	if utf8.RuneCountInString(message) > MaxChatMessageRunes {
		return ErrChatMessageTooLong
	}
	return nil
}

// maxConsultFreeTextRunes は、自由記述の上限（フロントの入力欄と同じ）。
const maxConsultFreeTextRunes = 1000

// IsValidConsultIntent は、受け付ける意図か。空（送っていない）は呼び出し側で別に扱う。
func IsValidConsultIntent(v string) bool {
	switch v {
	case ConsultIntentFindJob, ConsultIntentLearnIndustry, ConsultIntentHowToProceed, ConsultIntentOrganizeStrengths, ConsultIntentFree:
		return true
	}
	return false
}

// consultIntentLabel は、意図を質問の文脈に入れるときの文言。
func consultIntentLabel(v string) string {
	switch v {
	case ConsultIntentFindJob:
		return "やりたい仕事を一緒に考えたい"
	case ConsultIntentLearnIndustry:
		return "志望する業界について知りたい"
	case ConsultIntentHowToProceed:
		return "就活の進め方がわからない"
	case ConsultIntentOrganizeStrengths:
		return "自分の強みを整理したい"
	case ConsultIntentFree:
		return "自分の言葉で相談内容を入力した"
	}
	return ""
}

// ValidateConsultIntent は、リクエストの相談の意図を検証する。送っていない（空）は、既存のクライアントなので通す。
func ValidateConsultIntent(intent, message string) error {
	if intent == "" {
		return nil
	}
	if !IsValidConsultIntent(intent) {
		return errors.New("invalid consult_intent")
	}
	if intent == ConsultIntentFree {
		text := strings.TrimSpace(message)
		if text == "" {
			return errors.New("consult_intent=free requires a message")
		}
		if utf8.RuneCountInString(text) > maxConsultFreeTextRunes {
			return errors.New("consult_intent=free message is too long")
		}
	}
	return nil
}

// consultIntentContext は、質問生成のプロンプトに足す「相談の目的」。
// 入れるのは固定のラベルだけで、自由記述の本文は入れない（外部 AI へ個人情報を送らないため）。
// 未選択・取得できないときは空文字（従来どおりの質問になる）。
func (s *ChatService) consultIntentContext(sessionID string) string {
	if s.conversationContextRepo == nil {
		return ""
	}
	intent, err := s.conversationContextRepo.GetConsultIntent(sessionID)
	if err != nil || consultIntentLabel(intent) == "" {
		return ""
	}
	return "\n## 相談の目的\n学生は「" + consultIntentLabel(intent) + "」と相談を始めました。質問は、この目的に役立つ観点を優先してください。\n"
}
