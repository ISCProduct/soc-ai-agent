package interview

import (
	"context"
	"time"

	"Backend/internal/aifailover"
	"Backend/internal/openai"
)

// failoverTurnBudget はローカルへ縮退できるときの1ターンの上限（#1833）。
// フロントはターン全体を90秒で打ち切る（useInterviewSession.ts の TURN_FETCH_TIMEOUT_MS）。
// それより先に終え、縮退した結果（遅れた工程は言い換え応答・音声なし）を返すため。
const failoverTurnBudget = 80 * time.Second

// localSTTModel は観測ログ（stt observation）でローカルの文字起こしを示すモデル名。
// OpenAI のモデルの品質・速度の集計に混ぜないため。
const localSTTModel = "local:whisper.cpp"

// newSpeechFailover は OpenAI の障害時の縮退（#1833）を用意する。クライアントが無ければ nil。
// *openai.Client の nil をインターフェースへ入れると nil 判定が効かないので、ここで弾く。
func newSpeechFailover(client *openai.Client) *aifailover.Speech {
	if client == nil {
		return nil
	}
	return aifailover.New(client, aifailover.ConfigFromEnv())
}

// withTurnBudget は縮退が有効なときだけ、ターン全体に期限を付ける。
func (s *InterviewService) withTurnBudget(ctx context.Context) (context.Context, context.CancelFunc) {
	if !s.speech.Enabled() {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, failoverTurnBudget)
}

// transcribe は STT を呼ぶ。
func (s *InterviewService) transcribe(ctx context.Context, audio []byte, filename, hints string) (string, aifailover.Outcome, error) {
	if s.speech == nil {
		text, err := s.openaiClient.TranscribeWithHints(ctx, audio, filename, hints)
		return text, aifailover.Outcome{PrimaryTried: true}, err
	}
	return s.speech.Transcribe(ctx, audio, filename, hints)
}

// chatInterview は面接官の応答を作る。
func (s *InterviewService) chatInterview(ctx context.Context, systemPrompt string, history []map[string]string) (string, aifailover.Outcome, error) {
	if s.speech == nil {
		text, err := s.openaiClient.ChatInterview(ctx, systemPrompt, history)
		return text, aifailover.Outcome{PrimaryTried: true}, err
	}
	return s.speech.Chat(ctx, systemPrompt, history)
}

// speak は読み上げの音声を作る。ローカルは WAV を返す。
func (s *InterviewService) speak(ctx context.Context, text, voice string) ([]byte, aifailover.Outcome, error) {
	if s.speech == nil {
		audio, err := s.openaiClient.TTS(ctx, text, voice)
		return audio, aifailover.Outcome{PrimaryTried: true}, err
	}
	return s.speech.Speak(ctx, text, voice)
}

// fallbackStages はローカルで処理した工程を返す（応答のメタ ai_fallback）。
func fallbackStages(stt, llm, tts aifailover.Outcome) []string {
	var out []string
	if stt.Local {
		out = append(out, aifailover.StageSTT)
	}
	if llm.Local {
		out = append(out, aifailover.StageLLM)
	}
	if tts.Local {
		out = append(out, aifailover.StageTTS)
	}
	return out
}
