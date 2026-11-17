package resume

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
	"Backend/internal/services/piimask"
)

// 履歴書の本文は、外部の LLM へ送る前に必ず伏字にする。ここは、その回帰テスト。
// データはすべて合成。実在の個人情報は使わない（公開リポジトリ）。

type subjectStub struct {
	subj piimask.Subject
	err  error
}

func (s subjectStub) SubjectFor(uint) (piimask.Subject, error) { return s.subj, s.err }

const (
	piiName    = "山田太郎"
	piiEmail   = "taro.yamada@example.com"
	piiPhone   = "090-1234-5678"
	piiAddress = "東京都港区六本木1-2-3"
	piiSchool  = "架空情報専門学校"
)

var testSubject = piimask.Subject{Name: "山田 太郎", Email: piiEmail, SchoolName: piiSchool}

// piiBlocks は、連絡先の欄（行ごと除く）と、本文の中に個人情報が混ざる行を持つ。BBox はブロックごとに違う。
func piiBlocks() []models.ResumeTextBlock {
	texts := []string{
		"氏名 " + piiName,
		"電話 " + piiPhone,
		"現住所 " + piiAddress,
		"自己PR: " + piiName + "は、チームで10名のWeb開発を進めました。連絡は " + piiEmail + " まで。",
		"志望動機: " + piiAddress + " の寮から通える御社で、売上を伸ばす仕事がしたいです。",
		piiSchool + "で情報を学び、卒業制作でAPIを設計しました。",
	}
	out := make([]models.ResumeTextBlock, 0, len(texts))
	for i, t := range texts {
		out = append(out, models.ResumeTextBlock{
			PageNumber: 1, BlockIndex: i + 1, Text: t,
			BBox: `{"bbox":[10,` + string(rune('0'+i)) + `0,200,` + string(rune('0'+i)) + `9],"page_width":1240,"page_height":1755}`,
		})
	}
	return out
}

var piiNeedles = []string{piiName, "山田", "太郎", "taro.yamada", "example.com", "090-1234", piiPhone, "六本木", piiSchool}

func assertNoPII(t *testing.T, label, got string) {
	t.Helper()
	for _, n := range piiNeedles {
		if strings.Contains(got, n) {
			t.Errorf("%s に個人情報 %q が残っている:\n%s", label, n, got)
		}
	}
}

func TestMaskBlocksForLLM_DropsContactFieldsAndMasksBody(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	svc.SetSubjectReader(subjectStub{subj: testSubject})

	masked, err := svc.maskBlocksForLLM(1, piiBlocks())
	if err != nil {
		t.Fatalf("maskBlocksForLLM: %v", err)
	}
	text := buildResumeText(masked, 100000)
	assertNoPII(t, "LLM へ渡す本文", text)

	// 連絡先の欄（氏名・電話・現住所）は行ごと無い。本文は残り、伏字の記号が入る。
	if len(masked.list()) != 3 {
		t.Errorf("ブロック数 = %d, want 3（欄の3行を除いた本文）", len(masked.list()))
	}
	for _, want := range []string{"チームで10名のWeb開発", "売上を伸ばす仕事", "卒業制作でAPIを設計", "<NAME>", "<EMAIL>", "<ADDRESS>", "<SCHOOL>"} {
		if !strings.Contains(text, want) {
			t.Errorf("本文に %q が無い（レビューに必要な内容まで消えた、または伏字の記号が入っていない）:\n%s", want, text)
		}
	}
	// 位置と番号は変わらない（指摘は番号で元のブロックに対応づく）。
	orig := piiBlocks()
	for _, b := range masked.list() {
		var found bool
		for _, o := range orig {
			if o.BlockIndex == b.BlockIndex {
				found = true
				if o.BBox != b.BBox || o.PageNumber != b.PageNumber {
					t.Errorf("ブロック %d の位置・ページが変わった", b.BlockIndex)
				}
			}
		}
		if !found {
			t.Errorf("元に無いブロック番号 %d", b.BlockIndex)
		}
	}
}

// 登録情報を読めなかったら、伏字の準備ができない。本文を外へ送ってはならない（失敗したら送らない）。
func TestMaskBlocksForLLM_FailsClosedWhenSubjectUnavailable(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	svc.SetSubjectReader(subjectStub{err: errors.New("db down")})

	masked, err := svc.maskBlocksForLLM(1, piiBlocks())
	if !errors.Is(err, errMaskUnavailable) {
		t.Fatalf("err = %v, want errMaskUnavailable", err)
	}
	if len(masked.list()) != 0 {
		t.Errorf("失敗したのに本文が返っている: %d ブロック", len(masked.list()))
	}
}

// 登録情報を読む処理を注入していなくても、規則（メール・電話・住所）では伏字にする。
func TestMaskBlocksForLLM_RulesOnlyWhenNoSubjectReader(t *testing.T) {
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), nil)
	masked, err := svc.maskBlocksForLLM(1, piiBlocks())
	if err != nil {
		t.Fatal(err)
	}
	text := buildResumeText(masked, 100000)
	for _, n := range []string{"taro.yamada", "example.com", "090-1234", "六本木"} {
		if strings.Contains(text, n) {
			t.Errorf("規則だけでも %q は伏字になるはず:\n%s", n, text)
		}
	}
}

// 実際に AI へ送られるリクエスト本文（やり直しを含むすべて）に、個人情報が無いこと。
// あわせて、伏字済みの引用でも、指摘が元のブロックの位置に対応づくこと。
func TestBuildReviewScoreItems_RequestHasNoPII_AndItemsMapToOriginalBlocks(t *testing.T) {
	var mu sync.Mutex
	var bodies []string
	quotes := []string{
		"自己PR: <NAME>は、チームで10名のWeb開発を進めました。連絡は <EMAIL> まで。",
		"志望動機: <ADDRESS> の寮から通える御社で、売上を伸ばす仕事がしたいです。",
		"<SCHOOL>で情報を学び、卒業制作でAPIを設計しました。",
	}
	resp := responsesBody(t, reviewJSONItems(t, rubricScores(5, 4, 4, 5, 4), "指摘", quotes), "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(b))
		mu.Unlock()
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(srv.Close)
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	svc.SetSubjectReader(subjectStub{subj: testSubject})

	masked, err := svc.maskBlocksForLLM(1, piiBlocks())
	if err != nil {
		t.Fatal(err)
	}
	_, items, err := svc.buildReviewScoreItems(context.Background(), masked, "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("buildReviewScoreItems: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("AI へのリクエストが記録されていない")
	}
	for i, b := range bodies {
		assertNoPII(t, "AI へのリクエスト本文", b)
		if i == 0 && !strings.Contains(b, "EMAIL") {
			t.Errorf("伏字の記号が入っていない:\n%s", b)
		}
	}

	if len(items) != len(quotes) {
		t.Fatalf("指摘 = %d 件, want %d", len(items), len(quotes))
	}
	wantBBox := map[int]string{}
	for _, b := range piiBlocks() {
		wantBBox[b.BlockIndex] = b.BBox
	}
	// 指摘の位置は、伏字にする前の元のブロック（4・5・6番目）と同じ。
	got := map[string]bool{}
	for _, it := range items {
		got[it.BBox] = true
	}
	for _, idx := range []int{4, 5, 6} {
		if !got[wantBBox[idx]] {
			t.Errorf("ブロック %d の位置に指摘が対応づいていない", idx)
		}
	}
	// 注釈やDBに入る引用も、個人情報を含まない（伏字の記号のまま）。
	for _, it := range items {
		assertNoPII(t, "指摘の引用", it.Quote)
	}
}
