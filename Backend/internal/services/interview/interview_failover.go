package interview

import (
	"context"

	"Backend/internal/aifailover"
	"Backend/internal/openai"
)

// newSpeechFailover は OpenAI の障害時の縮退（#1833）を用意する。クライアントが無ければ nil。
// *openai.Client の nil をインターフェースへ入れると nil 判定が効かないので、ここで弾く。
func newSpeechFailover(client *openai.Client) *aifailover.Speech {
	if client == nil {
		return nil
	}
	return aifailover.New(client, aifailover.ConfigFromEnv())
}

// transcribe は STT を呼ぶ。local はローカルで処理したら true。
func (s *InterviewService) transcribe(ctx context.Context, audio []byte, filename, hints string) (text string, local bool, err error) {
	if s.speech == nil {
		text, err = s.openaiClient.TranscribeWithHints(ctx, audio, filename, hints)
		return text, false, err
	}
	return s.speech.Transcribe(ctx, audio, filename, hints)
}

// chatInterview は面接官の応答を作る。
func (s *InterviewService) chatInterview(ctx context.Context, systemPrompt string, history []map[string]string) (text string, local bool, err error) {
	if s.speech == nil {
		text, err = s.openaiClient.ChatInterview(ctx, systemPrompt, history)
		return text, false, err
	}
	return s.speech.Chat(ctx, systemPrompt, history)
}

// speak は読み上げの音声を作る。ローカルは WAV を返す。
func (s *InterviewService) speak(ctx context.Context, text, voice string) (audio []byte, local bool, err error) {
	if s.speech == nil {
		audio, err = s.openaiClient.TTS(ctx, text, voice)
		return audio, false, err
	}
	return s.speech.Speak(ctx, text, voice)
}

// fallbackStages はローカルで処理した工程を返す（応答のメタ ai_fallback）。
func fallbackStages(stt, llm, tts bool) []string {
	var out []string
	if stt {
		out = append(out, aifailover.StageSTT)
	}
	if llm {
		out = append(out, aifailover.StageLLM)
	}
	if tts {
		out = append(out, aifailover.StageTTS)
	}
	return out
}
