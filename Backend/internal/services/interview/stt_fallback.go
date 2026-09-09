package interview

import (
	"regexp"
	"strings"
	"time"
)

// FallbackReason はなぜ高精度モデルへ再送したか。
// 空文字は再送しないことを意味する。
type FallbackReason string

const (
	FallbackNone           FallbackReason = ""
	FallbackError          FallbackReason = "stt_error"
	FallbackEmpty          FallbackReason = "empty_result"
	FallbackUnintelligible FallbackReason = "unintelligible"
	FallbackTooShort       FallbackReason = "too_short_for_duration"
	FallbackRoleConfusion  FallbackReason = "role_confusion"
)

// minCharsPerSecond は「音声長に対してこれだけの文字は出るはず」という下限。
//
// 日本語の自然な発話はおおむね 4〜7文字/秒。1.0 は「明らかに何も取れていない」
// 水準に絞っている。ここを上げると通常発話まで再送し、費用が跳ねる。
const minCharsPerSecond = 1.0

// minDurationForLengthCheck は長さ判定を適用する最短の音声秒数。
//
// 短い相槌（「はい」）は正しく認識できていても文字数が少ない。
// 短い音声にまで下限を当てると、正常な発話を再送し続ける。
const minDurationForLengthCheck = 3.0

// unintelligibleMarkers は「聞き取れなかった」に相当する認識結果。
// 既存の空文字フォールバック（interview_turn.go）が入れる文言も含む。
var unintelligibleMarkers = []string{
	"（聞き取れませんでした）",
	"(聞き取れませんでした)",
	"聞き取れませんでした",
}

// roleConfusionPattern は面接で意味が変わる取り違え。
//
// mini は「御社」を「本社」と認識することが実測で8回中8回あった（Task 4）。
// 学生が自社を「本社」と呼ぶ場面は面接ではまず無く、
// 出たら誤変換を疑って高精度モデルで取り直す価値がある。
//
// 「本社」を含む正当な発話（「本社が東京にあります」等）も再送対象になるが、
// 再送しても認識結果が変わらないだけで害はない。
var roleConfusionPattern = regexp.MustCompile(`本社`)

// NeedsSTTFallback は高精度モデルへ再送すべきかを判定する（音声R&D Task 5）。
//
// 通常の発話は再送しない。再送率がそのまま追加費用になるため、
// 「問題が疑われる」ことが具体的に示せる場合だけに絞る。
//
// hadError は Transcribe 自体が失敗したか。text はその結果。
// durationSec は音声のおおよその長さ（0なら長さ判定を行わない）。
func NeedsSTTFallback(text string, hadError bool, durationSec float64) FallbackReason {
	if hadError {
		return FallbackError
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return FallbackEmpty
	}
	for _, m := range unintelligibleMarkers {
		if strings.Contains(trimmed, m) {
			return FallbackUnintelligible
		}
	}
	if durationSec >= minDurationForLengthCheck &&
		float64(len([]rune(trimmed))) < durationSec*minCharsPerSecond {
		return FallbackTooShort
	}
	if roleConfusionPattern.MatchString(trimmed) {
		return FallbackRoleConfusion
	}
	return FallbackNone
}

// ShouldUseFallbackResult は再送結果を採用すべきかを判定する。
//
// 再送しても失敗した、あるいは結果が空なら元の結果を残す。
// 「再認識したら悪化した」を防ぐため、元と再送を混同せず明示的に選ぶ。
func ShouldUseFallbackResult(retried string, retryErr error) bool {
	if retryErr != nil {
		return false
	}
	return strings.TrimSpace(retried) != ""
}

// FallbackModel は再送に使う高精度モデル名。
// 既定モデルが既に高精度なら再送しても意味が無いので、
// 呼び出し側は SameAsPrimary で判断する。
const FallbackModel = "gpt-4o-transcribe"

// sttFallbackTimeout は再送の打ち切り時間。
//
// 通常のSTTは60秒待つが、再送は元の結果があるうえでの上積みなので、
// 待つほど面接の体感が悪くなる。打ち切っても元の結果で会話は続く。
const sttFallbackTimeout = 20 * time.Second

// FallbackIsRedundant は既定モデルが既に再送先と同じかを返す。
// 同じなら再送しても結果は変わらず、費用だけが倍になる。
func FallbackIsRedundant() bool {
	return STTModelName() == FallbackModel
}
