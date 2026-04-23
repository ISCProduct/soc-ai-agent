package services

// ICS 行折り返しのテスト（Issue #320）
// 実行: cd Backend && go test ./internal/services/... -run TestFoldICSLine -v

import (
	"strings"
	"testing"
)

// TestFoldICSLine_ShortLine は75オクテット以下の行が折り返されないことを検証する
func TestFoldICSLine_ShortLine(t *testing.T) {
	line := "SUMMARY:短いタイトル"
	result := foldICSLine(line)
	if !strings.HasSuffix(result, "\r\n") {
		t.Error("行末に CRLF が付与されていない")
	}
	// 折り返しなし: CRLF は末尾の1箇所のみ
	if strings.Count(result, "\r\n") != 1 {
		t.Errorf("短い行が折り返された: %q", result)
	}
}

// TestFoldICSLine_LongLine は75オクテット超の行が折り返されることを検証する（#320修正の担保）
func TestFoldICSLine_LongLine(t *testing.T) {
	// 100文字のASCII行（100オクテット）
	long := "SUMMARY:" + strings.Repeat("A", 100)
	result := foldICSLine(long)
	if strings.Count(result, "\r\n") < 2 {
		t.Error("長い行が折り返されていない")
	}
	// 折り返し後の各行が75オクテット以下であることを確認
	lines := strings.Split(result, "\r\n")
	for i, l := range lines {
		if i == len(lines)-1 && l == "" {
			continue // 末尾の空行はスキップ
		}
		if len([]byte(l)) > 75 {
			t.Errorf("行 %d が75オクテット超: %d オクテット", i, len([]byte(l)))
		}
	}
}

// TestFoldICSLine_MultiByteLine はマルチバイト文字（日本語）で正しくオクテット数を計算することを検証する
func TestFoldICSLine_MultiByteLine(t *testing.T) {
	// 日本語 1文字 = 3オクテット（UTF-8）
	// "SUMMARY:" = 8オクテット + 日本語22文字 = 8 + 66 = 74オクテット（折り返しなし）
	shortJP := "SUMMARY:" + strings.Repeat("あ", 22) // 8 + 66 = 74 <= 75
	result := foldICSLine(shortJP)
	if strings.Count(result, "\r\n") != 1 {
		t.Errorf("74オクテットの行が折り返された: %q", result)
	}

	// "SUMMARY:" = 8オクテット + 日本語23文字 = 8 + 69 = 77オクテット（折り返しあり）
	longJP := "SUMMARY:" + strings.Repeat("あ", 23) // 8 + 69 = 77 > 75
	result2 := foldICSLine(longJP)
	if strings.Count(result2, "\r\n") < 2 {
		t.Error("77オクテットの日本語行が折り返されていない")
	}

	// 折り返し後の各行が75オクテット以下
	lines := strings.Split(result2, "\r\n")
	for i, l := range lines {
		if i == len(lines)-1 && l == "" {
			continue
		}
		if len([]byte(l)) > 75 {
			t.Errorf("行 %d が75オクテット超: %d オクテット", i, len([]byte(l)))
		}
	}
}

// TestFoldICSLine_ContinuationLineStartsWithSpace は折り返し継続行がスペースで始まることを検証する（RFC 5545）
func TestFoldICSLine_ContinuationLineStartsWithSpace(t *testing.T) {
	long := "DESCRIPTION:" + strings.Repeat("X", 100)
	result := foldICSLine(long)
	lines := strings.Split(result, "\r\n")
	for i, l := range lines {
		if i == 0 || (i == len(lines)-1 && l == "") {
			continue
		}
		if !strings.HasPrefix(l, " ") {
			t.Errorf("継続行 %d がスペースで始まっていない: %q", i, l)
		}
	}
}

// TestFoldICSLine_ContentPreserved は折り返し後に内容が変化しないことを検証する
func TestFoldICSLine_ContentPreserved(t *testing.T) {
	original := "SUMMARY:" + strings.Repeat("テスト", 30) // 長い日本語行
	result := foldICSLine(original)

	// 折り返しを除去して元の内容と一致するか確認
	unfolded := strings.ReplaceAll(result, "\r\n ", "")
	unfolded = strings.TrimSuffix(unfolded, "\r\n")
	if unfolded != original {
		t.Errorf("折り返し後に内容が変化した\n got:  %q\n want: %q", unfolded, original)
	}
}
