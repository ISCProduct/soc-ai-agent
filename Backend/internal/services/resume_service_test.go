package services

// newSeekableReader のメモリ上限テスト（Issue #312）
// 実行: cd Backend && go test ./internal/services/... -run TestSeekableReader -v

import (
	"bytes"
	"io"
	"strings"
	"testing"
)

// TestNewSeekableReader_NormalFile は通常サイズのファイルが正常に読み込まれることを検証する
func TestNewSeekableReader_NormalFile(t *testing.T) {
	content := []byte("Hello, PDF content!")
	rc := io.NopCloser(bytes.NewReader(content))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Fatalf("通常ファイルでエラーが発生した: %v", err)
	}
	if sr == nil {
		t.Fatal("seekableReader が nil を返した")
	}

	// 読み込み内容の検証
	got, err := io.ReadAll(sr)
	if err != nil {
		t.Fatalf("読み込みエラー: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("内容が一致しない: got %q, want %q", got, content)
	}
}

// TestNewSeekableReader_OversizedFile は最大サイズを超えるファイルがエラーになることを検証する（#312修正の担保）
func TestNewSeekableReader_OversizedFile(t *testing.T) {
	// maxResumeBytes + 1 バイトのデータを生成
	oversized := bytes.Repeat([]byte("x"), maxResumeBytes+1)
	rc := io.NopCloser(bytes.NewReader(oversized))

	_, err := newSeekableReader(rc)
	if err == nil {
		t.Error("最大サイズを超えるファイルはエラーを返すべき")
	}
	if !strings.Contains(err.Error(), "exceeds maximum") {
		t.Errorf("エラーメッセージが不正: got %q", err.Error())
	}
}

// TestNewSeekableReader_ExactlyMaxSize は最大サイズちょうどのファイルが正常に読み込まれることを検証する
func TestNewSeekableReader_ExactlyMaxSize(t *testing.T) {
	exactMax := bytes.Repeat([]byte("x"), maxResumeBytes)
	rc := io.NopCloser(bytes.NewReader(exactMax))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Errorf("最大サイズちょうどはエラーになるべきではない: %v", err)
	}
	if sr == nil {
		t.Error("seekableReader が nil を返した")
	}
}

// TestNewSeekableReader_SeekWorks は Seek 操作が正しく動作することを検証する
func TestNewSeekableReader_SeekWorks(t *testing.T) {
	content := []byte("ABCDEFGHIJ")
	rc := io.NopCloser(bytes.NewReader(content))

	sr, err := newSeekableReader(rc)
	if err != nil {
		t.Fatalf("newSeekableReader returned error: %v", err)
	}

	// 5バイト目にシーク
	if _, err := sr.Seek(5, io.SeekStart); err != nil {
		t.Fatalf("Seek returned error: %v", err)
	}

	buf := make([]byte, 3)
	if _, err := sr.Read(buf); err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if string(buf) != "FGH" {
		t.Errorf("Seek後の読み込み結果が不正: got %q, want %q", buf, "FGH")
	}
}
