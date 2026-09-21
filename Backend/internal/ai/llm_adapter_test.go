package ai_test

// LLM アダプターレイヤーのコンパイル時インターフェース適合テスト (Issue #177)
//
// 実行: cd Backend && go test ./test/services/... -run LLMAdapter -v

import (
	"context"
	"testing"

	domainai "Backend/domain/ai"
	internalai "Backend/internal/ai"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// コンパイル時に FallbackAdapter が LLMClient インターフェースを満たすことを検証
var _ domainai.LLMClient = (*internalai.FallbackAdapter)(nil)

func TestFallbackAdapter_GenerateText(t *testing.T) {
	adapter := internalai.NewFallbackAdapter()
	text, err := adapter.GenerateText(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.NotEmpty(t, text, "フォールバックは空でないメッセージを返すこと")
}

func TestFallbackAdapter_GenerateJSON(t *testing.T) {
	adapter := internalai.NewFallbackAdapter()
	json, err := adapter.GenerateJSON(context.Background(), "system", "user")
	require.NoError(t, err)
	assert.NotEmpty(t, json, "フォールバックは有効な JSON を返すこと")
}

func TestFallbackAdapter_GenerateEmbedding_ReturnsError(t *testing.T) {
	adapter := internalai.NewFallbackAdapter()
	embedding, err := adapter.GenerateEmbedding(context.Background(), "test text")
	assert.Error(t, err, "フォールバックはエンべディングでエラーを返すこと")
	assert.Nil(t, embedding)
}

func TestFallbackAdapter_TranscribeAudio_ReturnsError(t *testing.T) {
	adapter := internalai.NewFallbackAdapter()
	text, err := adapter.TranscribeAudio(context.Background(), []byte("audio"), "test.wav")
	assert.Error(t, err, "フォールバックは音声認識でエラーを返すこと")
	assert.Empty(t, text)
}

// LLMClient を依存注入で受け取る関数のテスト
// サービス層がインターフェース経由で AI を呼び出せることを確認
func TestLLMClientDependencyInjection(t *testing.T) {
	// FallbackAdapter を LLMClient として注入
	var client domainai.LLMClient = internalai.NewFallbackAdapter()

	// TextClient として使用
	text, err := client.GenerateText(context.Background(), "sp", "up")
	require.NoError(t, err)
	assert.NotEmpty(t, text)

	// EmbeddingClient として使用（フォールバックはエラーを返す）
	_, embErr := client.GenerateEmbedding(context.Background(), "test")
	assert.Error(t, embErr, "フォールバック実装ではエンべディングは利用不可")
}
