package resume

import (
	"strings"
	"testing"

	"Backend/internal/models"
)

func ocrBlocks(texts ...string) []models.ResumeTextBlock {
	out := make([]models.ResumeTextBlock, 0, len(texts))
	for i, tx := range texts {
		out = append(out, models.ResumeTextBlock{PageNumber: 1, BlockIndex: i + 1, Text: tx, BBox: `[0,0,1,1]`})
	}
	return out
}

// OCR は行ごとにブロックを分ける。住所・電話・メールが複数のブロックにまたがっても、続きが残らないこと。合成データ。
func TestMaskBlocksForLLM_OCRSplitContactValues(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	svc.SetSubjectReader(subjectStub{subj: testSubject})
	tests := []struct {
		name  string
		texts []string
		leak  []string
	}{
		{"住所が2行(1行目に見出し)", []string{"現住所 東京都港区六本木", "1-2-3 〇〇マンション101", "志望動機: 開発がしたいです。"}, []string{"1-2-3", "マンション"}},
		{"住所が2行(見出しだけ)", []string{"住所", "東京都港区六本木", "1-2-3 〇〇マンション101", "志望動機: 開発がしたいです。"}, []string{"1-2-3", "マンション"}},
		{"電話の数字に空白が入る", []string{"電話", "0 9 0 - 1 2 3 4 - 5 6 7 8"}, []string{"1 2 3 4"}},
		{"本文中の電話に空白が入る", []string{"連絡は 0 9 0 1 2 3 4 5 6 7 8 まで"}, []string{"1 2 3 4"}},
		{"メールが2行に分かれる", []string{"自己PR: 連絡は taro.yamada@", "example.com まで"}, []string{"taro.yamada", "example.com"}},
		{"メールのドメインだけの行", []string{"@example.com"}, []string{"example.com"}},
		{"電話が2ブロック", []string{"連絡先 090-1234", "5678"}, []string{"5678"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked, err := svc.maskBlocksForLLM(1, ocrBlocks(tt.texts...))
			if err != nil {
				t.Fatal(err)
			}
			text := buildResumeText(masked, 100000)
			for _, l := range tt.leak {
				if strings.Contains(text, l) {
					t.Errorf("%q が残っている: %q", l, strings.ReplaceAll(text, "\n", " / "))
				}
			}
		})
	}
}

// 続きのブロックの除外が、学歴・職歴・資格の行を巻き込まないこと。
func TestMaskBlocksForLLM_ContinuationKeepsRealRows(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	svc.SetSubjectReader(subjectStub{subj: testSubject})
	masked, err := svc.maskBlocksForLLM(1, ocrBlocks(
		"住所", "東京都港区六本木1-2-3", "2020年4月 入学", "2023年3月 卒業", "2023年4月 株式会社サンプル 入社",
		"電話", "090-1234-5678", "資格", "TOEIC 780点", "10名のチームでリーダー",
	))
	if err != nil {
		t.Fatal(err)
	}
	text := buildResumeText(masked, 100000)
	for _, keep := range []string{"入学", "卒業", "入社", "TOEIC 780点", "10名のチームでリーダー"} {
		if !strings.Contains(text, keep) {
			t.Errorf("本文の %q が消えた:\n%s", keep, text)
		}
	}
}

// 帰省先の欄の続き・空のブロックを挟んだ値も残さないこと（レビューでの指摘）。合成データ。
func TestMaskBlocksForLLM_ReviewedSplitCases(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	svc.SetSubjectReader(subjectStub{subj: testSubject})
	for _, tt := range []struct {
		name  string
		texts []string
		leak  []string
	}{
		{"帰省先の住所が続く", []string{"帰省先 東京都港区", "六本木1-2-3", "ヒルズ棟 12階"}, []string{"六本木", "ヒルズ"}},
		{"見出しの次が空のブロック", []string{"氏名", "", "ヤマダ"}, []string{"ヤマダ"}},
		{"@で終わる行の次が空のブロック", []string{"連絡は taro@", "", "example.com まで"}, []string{"example.com"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			masked, err := svc.maskBlocksForLLM(1, ocrBlocks(tt.texts...))
			if err != nil {
				t.Fatal(err)
			}
			var all strings.Builder
			for _, b := range masked.list() {
				all.WriteString(b.Text + "\n")
			}
			for _, l := range tt.leak {
				if strings.Contains(all.String(), l) {
					t.Errorf("%q が残った: %q", l, all.String())
				}
			}
		})
	}
}
