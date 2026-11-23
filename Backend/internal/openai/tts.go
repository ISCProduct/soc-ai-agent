package openai

import (
	"Backend/internal/aimodel"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Transcribe は音声データを Whisper でテキストに変換します
func (cli *Client) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	if err := cli.ensureAudio(); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	WriteTranscribeContext(w, resolveTranscribeModel(), "")
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	err = w.Close()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.AudioBaseURL()+"/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.audioKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := cli.httpClientFor("audio", 60*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &StatusError{Format: "whisper error %d: %s", StatusCode: resp.StatusCode, Body: string(body)}
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return strings.TrimSpace(result.Text), nil
}

// DefaultTTSModel は TTS の既定モデル。
//
// 利用量の記録側（interview/turn_usage.go）と同じ値を見る必要がある。
// DefaultTranscribeModel と同じ理由で、文字列を書き写さず定数を共有する。
// 書き写すと片方だけ変わり、ログ上のモデル名と実際が食い違う。
const DefaultTTSModel = "tts-1"

// TTS は OpenAI TTS API でテキストを音声に変換し、mp3 バイト列を返します。
//
// 生成時間は文の長さにほぼ比例し、面接のターンでは最も遅い工程になる。
// 文の区切りで最大 ttsMaxChunks 個に分けて同時に作り、mp3 をそのままつなげる
// （mp3 はフレームの連続なので、つなげても1本の音声として再生できる）。
func (cli *Client) TTS(ctx context.Context, text, voice string) ([]byte, error) {
	if err := cli.ensureAudio(); err != nil {
		return nil, err
	}
	chunks := splitForTTS(text, ttsMaxChunks)
	if len(chunks) <= 1 {
		return cli.ttsOne(ctx, text, voice)
	}
	parts := make([][]byte, len(chunks))
	errs := make([]error, len(chunks))
	var wg sync.WaitGroup
	for i, chunk := range chunks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// panic を失敗として返す。握りつぶすと途中が欠けた音声をつなげて返してしまう
			defer func() {
				if r := recover(); r != nil {
					errs[i] = fmt.Errorf("tts chunk panic: %v", r)
				}
			}()
			parts[i], errs[i] = cli.ttsOne(ctx, chunk, voice)
		}()
	}
	wg.Wait()
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return bytes.Join(parts, nil), nil
}

// ttsMaxChunks は1回の読み上げで同時に呼ぶ上限。面接の応答は2〜3文（150文字以内）。
const ttsMaxChunks = 3

// splitForTTS は文末（。！？!?）で区切り、長さがなるべく揃うよう最大 max 個にまとめる。
// 区切りが無い・短い文は1つのまま返す（分けても速くならず、つなぎ目が増えるだけ）。
func splitForTTS(text string, max int) []string {
	var sentences []string
	var cur []rune
	for _, r := range text {
		cur = append(cur, r)
		if strings.ContainsRune("。！？!?", r) {
			sentences = append(sentences, string(cur))
			cur = nil
		}
	}
	if strings.TrimSpace(string(cur)) != "" {
		sentences = append(sentences, string(cur))
	} else if len(cur) > 0 && len(sentences) > 0 {
		sentences[len(sentences)-1] += string(cur)
	}
	if len(sentences) <= 1 || max <= 1 {
		return []string{text}
	}
	if len(sentences) <= max {
		return sentences
	}
	// 残りの文字数を残りの個数で割った長さを目安に、前から詰める
	total := utf8.RuneCountInString(text)
	var out []string
	var buf strings.Builder
	for i, s := range sentences {
		buf.WriteString(s)
		left := max - len(out)
		if left > 1 && i < len(sentences)-1 && utf8.RuneCountInString(buf.String()) >= total/max {
			out = append(out, buf.String())
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

// ttsOne は1回の API 呼び出しで読み上げる。
func (cli *Client) ttsOne(ctx context.Context, text, voice string) ([]byte, error) {

	model := os.Getenv("OPENAI_TTS_MODEL")
	if model == "" {
		model = DefaultTTSModel
	}
	if voice == "" {
		voice = os.Getenv("OPENAI_TTS_VOICE")
	}
	if voice == "" {
		voice = "alloy"
	}

	payload := map[string]any{
		"model": model,
		"input": text,
		"voice": voice,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.AudioBaseURL()+"/audio/speech", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+cli.audioKey)
	req.Header.Set("Content-Type", "application/json")

	client := cli.httpClientFor("audio", 60*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		return nil, &StatusError{Format: "tts error %d: %s", StatusCode: resp.StatusCode, Body: string(errBody)}
	}

	return io.ReadAll(resp.Body)
}

// ChatInterview は面接官として会話し、次のメッセージを返します
func (cli *Client) ChatInterview(ctx context.Context, systemPrompt string, history []map[string]string) (string, error) {
	// 面接の応答生成はテキスト系（音声ではない）
	if err := cli.ensureText(); err != nil {
		return "", err
	}
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type request struct {
		Model       string    `json:"model"`
		Messages    []message `json:"messages"`
		MaxTokens   int       `json:"max_tokens"`
		Temperature float32   `json:"temperature"`
	}

	// 面接の応答は学生に見える判断（上位の層）。OPENAI_INTERVIEW_MODEL → AI_MODEL_PREMIUM → gpt-4o-mini（#1526）
	model := aimodel.For(aimodel.StageInterviewResponse)

	msgs := []message{{Role: "system", Content: systemPrompt}}
	for _, h := range history {
		// role はクライアント（multipart の history フィールド）由来なので
		// ホワイトリストで落とす（#1600）。素通しだと学生が
		// {"role":"system","content":"..."} を送るだけで面接官の system 指示を
		// 上書きできる。区切りで囲む対策はこの型の注入には効かない。
		role := h["role"]
		if role != "user" && role != "assistant" {
			role = "user"
		}
		// 発話の履歴（学生の発話と、それを受けた面接官の応答）は、ガードがあれば伏字にして送る
		content, err := guardInput(ctx, h["content"])
		if err != nil {
			return "", err
		}
		msgs = append(msgs, message{Role: role, Content: content})
	}

	payload := request{
		Model:       model,
		Messages:    msgs,
		MaxTokens:   200,
		Temperature: 0.7,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.BaseURL()+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.textKey)
	req.Header.Set("Content-Type", "application/json")

	client := cli.httpClientFor("text", 60*time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &StatusError{Format: "chat error %d: %s", StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", errors.New("no choices returned from chat API")
	}
	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}
