package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"
)

// TranscribeWithHints は補助語（prompt）を添えて音声を文字起こしする（音声R&D Task 4）。
//
// hints が空なら Transcribe と同じ動作になる。企業未選択の面接でも
// 従来どおり動かすため、補助語の有無で経路を分けない。
//
// prompt は gpt-4o-transcribe / gpt-4o-mini-transcribe / whisper-1 の
// いずれも受理することを実APIで確認済み（RESULTS_stt_hints.md）。
//
// 実測では固有名詞の認識が改善し、無関係な語を渡しても幻覚は起きなかった。
// ただし mini は実行ごとのばらつきが大きく、補助語があっても
// 高精度モデルの安定性には届かない。
func (cli *Client) TranscribeWithHints(ctx context.Context, audio []byte, filename, hints string) (string, error) {
	return cli.TranscribeWithModel(ctx, audio, filename, hints, resolveTranscribeModel())
}

// TranscribeWithModel はモデルを明示して文字起こしする（音声R&D Task 5）。
//
// 選択的フォールバックで、既定モデルとは別の高精度モデルへ
// 同じ音声を再送するために使う。
func (cli *Client) TranscribeWithModel(ctx context.Context, audio []byte, filename, hints, model string) (string, error) {
	if err := cli.ensureAudio(); err != nil {
		return "", err
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("model", model)
	_ = w.WriteField("language", "ja")
	if strings.TrimSpace(hints) != "" {
		// 補助語は「認識してほしい語の一覧」であって文章ではない。
		// 文章を渡すと続きの文脈として扱われ、認識が引きずられる。
		_ = w.WriteField("prompt", hints)
	}
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cli.AudioBaseURL()+"/audio/transcriptions", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cli.audioKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		// 補助語には企業名が含まれる。エラー本文をそのまま返すと
		// リクエスト内容がログへ流れうるため、状態コードだけにする。
		return "", fmt.Errorf("transcription failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Text, nil
}

// DefaultTranscribeModel は STT の既定モデル。
// ずれると補助語ありと無しで別モデルが使われ、比較が成立しない。
const DefaultTranscribeModel = "gpt-4o-mini-transcribe"

// resolveTranscribeModel は環境変数を見て使用するSTTモデルを決める。
//
// Transcribe と TranscribeWithHints が別々に既定値を持っていた頃は、
// 片方だけ変えても誰も気づかず、ログに出るモデル名と実際に叩くモデルが
// 食い違いうる状態だった。解決はここ1箇所に集約する。
func resolveTranscribeModel() string {
	if m := os.Getenv("OPENAI_WHISPER_MODEL"); m != "" {
		return m
	}
	return DefaultTranscribeModel
}
