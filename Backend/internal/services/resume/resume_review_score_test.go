package resume

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"Backend/internal/models"
	"Backend/internal/openai"
)

// buildReviewScoreItems のスコア経路を、OpenAI をスタブしたサービス層で検証する（#1529）。
//
// ルーブリックの純関数テスト（resume_rubric_test.go）だけでは、
// 「総合スコアをサーバー側で算出して保存する」「違反ならスコア無しにする」
// 「上限到達なら切れたJSONを読まない」という配線が壊れても気付けない。

// aiStub は Responses API のスタブ。bodies を呼び出し順に返し、尽きたら最後を繰り返す。
// 呼び出しごとに要求された max_output_tokens を記録する（上限到達時のやり直し検証用）と、
// text.format.type（JSON mode の指定 / #1583）を記録する。
type aiStub struct {
	mu          sync.Mutex
	bodies      []string
	calls       int
	maxTokens   []int
	textFormats []string
}

func (s *aiStub) next(requestedMaxTokens int, textFormat string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	body := s.bodies[min(s.calls, len(s.bodies)-1)]
	s.calls++
	s.maxTokens = append(s.maxTokens, requestedMaxTokens)
	s.textFormats = append(s.textFormats, textFormat)
	return body
}

// requestedTextFormats は呼び出しごとの text.format.type を返す。
// text を送っていない呼び出しは空文字列になる。
func (s *aiStub) requestedTextFormats() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.textFormats)
}

func (s *aiStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *aiStub) requestedMaxTokens() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.maxTokens)
}

// newReviewService は OpenAI をスタブに向けた ResumeService を返す。
func newReviewService(t *testing.T, bodies ...string) (*ResumeService, *aiStub) {
	t.Helper()
	stub := &aiStub{bodies: bodies}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxOutputTokens int `json:"max_output_tokens"`
			Text            struct {
				Format struct {
					Type string `json:"type"`
				} `json:"format"`
			} `json:"text"`
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &req)
		_, _ = w.Write([]byte(stub.next(req.MaxOutputTokens, req.Text.Format.Type)))
	}))
	t.Cleanup(srv.Close)
	svc := NewResumeService(&resumeRepoStub{}, t.TempDir(), openai.NewWithBaseURL(srv.URL, "gpt-4o-mini"))
	return svc, stub
}

// responsesBody は Responses API の応答を組み立てる。
// incompleteReason に "max_output_tokens" を入れると上限到達を再現する。
func responsesBody(t *testing.T, outputText, incompleteReason string) string {
	t.Helper()
	payload := map[string]any{"output_text": outputText}
	if incompleteReason != "" {
		payload["incomplete_details"] = map[string]string{"reason": incompleteReason}
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("スタブ応答の組み立てに失敗: %v", err)
	}
	return string(encoded)
}

// reviewJSON はレビュー結果のJSON（LLM の出力に相当）を組み立てる。
// 指摘は本文ブロックの先頭3件に紐づく。
func reviewJSON(t *testing.T, scores map[string]int) string {
	t.Helper()
	return reviewJSONItems(t, scores, "指摘", blockTexts[:3])
}

// reviewJSONItems は指摘の件数と文言を指定してレビュー結果のJSONを組み立てる。
func reviewJSONItems(t *testing.T, scores map[string]int, message string, quotes []string) string {
	t.Helper()
	items := make([]map[string]any, 0, len(quotes))
	for _, quote := range quotes {
		items = append(items, map[string]any{
			"quote": quote, "message": message, "suggestion": "改善案", "severity": "info",
		})
	}
	payload := map[string]any{"summary": "確認しました", "items": items}
	if scores != nil {
		payload["scores"] = scores
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("レビューJSONの組み立てに失敗: %v", err)
	}
	return string(encoded)
}

// blockTexts は本文ブロックの文面。引用の照合が一意に決まるよう内容を離している。
var blockTexts = []string{
	"売上を前年比120%に伸ばしました",
	"3名のチームでリーダーを担当しました",
	"基本情報技術者試験に合格しています",
	"TOEICで800点を取得しました",
	"Webサイトの運用を2年間担当しました",
}

func reviewBlocks() []models.ResumeTextBlock {
	texts := blockTexts
	blocks := make([]models.ResumeTextBlock, 0, len(texts))
	for i, text := range texts {
		blocks = append(blocks, models.ResumeTextBlock{
			PageNumber: 1,
			BlockIndex: i + 1,
			Text:       text,
			BBox:       `[0,0,100,20]`,
		})
	}
	return blocks
}

// TestBuildReviewScoreItems_ScoreComputedServerSide は項目スコアから総合スコアが
// サーバー側で算出され、内訳が保存されることを検証する。
func TestBuildReviewScoreItems_ScoreComputedServerSide(t *testing.T) {
	tests := []struct {
		name          string
		candidateType string
		scores        map[string]int
		wantScore     int
	}{
		{
			name:          "新卒",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(5, 4, 4, 5, 4),
			wantScore:     90,
		},
		{
			name:          "中途は同じ項目スコアでも重みが違う",
			candidateType: candidateTypeMidCareer,
			scores:        rubricScores(5, 4, 4, 5, 4),
			wantScore:     87,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, stub := newReviewService(t, responsesBody(t, reviewJSON(t, tt.scores), ""))

			review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", tt.candidateType, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(items) == 0 {
				t.Fatal("指摘が1件も紐づいていない（テストの前提が崩れている）")
			}
			if review.Score == nil {
				t.Fatal("Score = nil, want 値あり")
			}
			if *review.Score != tt.wantScore {
				t.Errorf("Score = %d, want %d", *review.Score, tt.wantScore)
			}
			if review.ItemScoresJSON == nil {
				t.Fatal("ItemScoresJSON = nil, want 内訳あり")
			}
			var saved map[string]int
			if err := json.Unmarshal([]byte(*review.ItemScoresJSON), &saved); err != nil {
				t.Fatalf("内訳のJSONが壊れている: %v", err)
			}
			for key, want := range tt.scores {
				if saved[key] != want {
					t.Errorf("内訳 %s = %d, want %d", key, saved[key], want)
				}
			}
			// スコアが正当なら作り直しは起きない（1回だけ呼ぶ）
			if got := stub.count(); got != 1 {
				t.Errorf("AI 呼び出し回数 = %d, want 1", got)
			}
		})
	}
}

// TestBuildReviewScoreItems_RubricViolationHasNoScore はルーブリック違反のとき
// 固定値を入れず、スコアと内訳をどちらも空にすることを検証する。
// 講評と指摘事項は残す（学生への価値をゼロにしない）。
func TestBuildReviewScoreItems_RubricViolationHasNoScore(t *testing.T) {
	tests := []struct {
		name   string
		scores map[string]int
	}{
		{name: "scoresが無い", scores: nil},
		{name: "項目欠落", scores: map[string]int{"specificity": 4, "achievement": 3}},
		{name: "100点満点で返された", scores: rubricScores(80, 90, 70, 60, 100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 初回・やり直しの両方が違反
			body := responsesBody(t, reviewJSON(t, tt.scores), "")
			svc, stub := newReviewService(t, body, body)

			review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if review.Score != nil {
				t.Errorf("Score = %d, want nil（固定値を入れない）", *review.Score)
			}
			if review.ItemScoresJSON != nil {
				t.Errorf("ItemScoresJSON = %q, want nil（総合スコアと内訳は揃えて落とす）", *review.ItemScoresJSON)
			}
			if review.Summary == "" {
				t.Error("講評まで捨ててはいけない")
			}
			if len(items) == 0 {
				t.Error("指摘事項まで捨ててはいけない")
			}
			// スコアだけが不正なときも作り直しは1度だけ
			if got := stub.count(); got != 2 {
				t.Errorf("AI 呼び出し回数 = %d, want 2（初回＋やり直し1回）", got)
			}
		})
	}
}

// TestBuildReviewScoreItems_RetryFixesScore はやり直しで正当なスコアが返れば採用することを検証する。
func TestBuildReviewScoreItems_RetryFixesScore(t *testing.T) {
	svc, stub := newReviewService(t,
		responsesBody(t, reviewJSON(t, map[string]int{"specificity": 4}), ""), // 初回は項目欠落
		responsesBody(t, reviewJSON(t, rubricScores(3, 3, 3, 3, 3)), ""),      // やり直しは正常
	)

	review, _, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if review.Score == nil || *review.Score != 60 {
		t.Fatalf("Score = %v, want 60", review.Score)
	}
	if got := stub.count(); got != 2 {
		t.Errorf("AI 呼び出し回数 = %d, want 2", got)
	}
}

// TestBuildReviewScoreItems_RetryKeepsInitialItems はスコアだけが不正なときに
// 初回の指摘集合を守ることを検証する。
//
// やり直しはブロック一覧ベースの別プロンプトで、件数が増えても紐づきの質が
// 上がる保証が無い。採点のやり直しのために本文へ良く紐づいた指摘を賭けない。
func TestBuildReviewScoreItems_RetryKeepsInitialItems(t *testing.T) {
	svc, stub := newReviewService(t,
		// 初回: スコアだけ不正。指摘は3件紐づく
		responsesBody(t, reviewJSONItems(t, map[string]int{"specificity": 4}, "初回の指摘", blockTexts[:3]), ""),
		// やり直し: スコアは正常。指摘は5件（件数だけなら初回より多い）
		responsesBody(t, reviewJSONItems(t, rubricScores(3, 3, 3, 3, 3), "やり直しの指摘", blockTexts), ""),
	)

	review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// スコアはやり直しの分を採る
	if review.Score == nil || *review.Score != 60 {
		t.Fatalf("Score = %v, want 60（やり直しのスコアを採用）", review.Score)
	}
	// 指摘は初回のまま
	if len(items) != 3 {
		t.Errorf("指摘件数 = %d, want 3（初回の集合を保つ）", len(items))
	}
	for _, item := range items {
		if item.Message != "初回の指摘" {
			t.Errorf("指摘が差し替わっている: %q", item.Message)
		}
	}
	if got := stub.count(); got != 2 {
		t.Errorf("AI 呼び出し回数 = %d, want 2", got)
	}
}

// TestBuildReviewScoreItems_RetryFillsMissingItems は初回の指摘が3件未満のときは
// 従来どおりやり直しの結果で件数を増やすことを検証する（#1529 で壊していないこと）。
func TestBuildReviewScoreItems_RetryFillsMissingItems(t *testing.T) {
	svc, _ := newReviewService(t,
		// 初回: 紐づく指摘が1件だけ（スコアは正常）
		responsesBody(t, reviewJSONItems(t, rubricScores(3, 3, 3, 3, 3), "初回の指摘", blockTexts[:1]), ""),
		// やり直し: 5件
		responsesBody(t, reviewJSONItems(t, rubricScores(3, 3, 3, 3, 3), "やり直しの指摘", blockTexts), ""),
	)

	_, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != len(blockTexts) {
		t.Fatalf("指摘件数 = %d, want %d（やり直しで増やす）", len(items), len(blockTexts))
	}
	if items[0].Message != "やり直しの指摘" {
		t.Errorf("やり直しの指摘が採用されていない: %q", items[0].Message)
	}
}

// TestBuildReviewScoreItems_TruncatedOutputRetriesWithDoubleBudget は出力が上限で
// 切れたとき、枠を倍にして1度だけやり直すことを検証する（#1521 の定石）。
//
// クライアント内部の「枠を倍にして再試行」は本文が空のときしか発火しないため、
// 本文が途中まで返る切れ方はここでしか救えない。
func TestBuildReviewScoreItems_TruncatedOutputRetriesWithDoubleBudget(t *testing.T) {
	svc, stub := newReviewService(t,
		responsesBody(t, truncatedReviewJSON, "max_output_tokens"),       // 1回目: 2400 で切れる
		responsesBody(t, reviewJSON(t, rubricScores(3, 3, 3, 3, 3)), ""), // 2回目: 4800 で収まる
	)

	review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err != nil {
		t.Fatalf("枠を倍にしたやり直しで成功するべき: %v", err)
	}
	if review.Score == nil || *review.Score != 60 {
		t.Errorf("Score = %v, want 60", review.Score)
	}
	if len(items) == 0 {
		t.Error("指摘が紐づいていない")
	}
	if got := stub.requestedMaxTokens(); !slices.Equal(got, []int{ReviewMaxOutputTokens, ReviewMaxOutputTokens * 2}) {
		t.Errorf("要求した出力上限 = %v, want [%d %d]", got, ReviewMaxOutputTokens, ReviewMaxOutputTokens*2)
	}
}

// TestBuildReviewScoreItems_TruncatedOutputFails は枠を倍にしても切れるなら
// レビューごと失敗させることを検証する。
//
// **このテストが固定しているのは文言と呼び出し回数であって、
// 「部分的なレビューが保存されない」ことではない。** 現在のスキーマは items が
// 最後のフィールドなので、どこで切れても外側の '{' と '[' が閉じず decodeJSON は
// 必ず失敗する（チェックを外しても失敗する。変わるのはエラー文言だけ）。
// 上限到達の検知は「原因が分かる文言を出すこと」と、items を最後以外へ動かす
// スキーマ変更に対する保険の2点が目的である。
func TestBuildReviewScoreItems_TruncatedOutputFails(t *testing.T) {
	body := responsesBody(t, truncatedReviewJSON, "max_output_tokens")
	svc, stub := newReviewService(t, body, body)

	review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err == nil {
		t.Fatalf("エラーが返るべき（review=%+v items=%d）", review, len(items))
	}
	if !errors.Is(err, errReviewOutputTruncated) {
		t.Errorf("上限到達のエラーであるべき: %v", err)
	}
	if !strings.Contains(err.Error(), "切れ") {
		t.Errorf("エラーメッセージが上限到達を伝えていない: %v", err)
	}
	if review != nil || items != nil {
		t.Error("切れた出力から部分的な結果を作ってはいけない")
	}
	// 初回＋枠を倍にしたやり直しの2回で打ち切る（同じ枠での再試行はしない）
	if got := stub.count(); got != 2 {
		t.Errorf("AI 呼び出し回数 = %d, want 2", got)
	}
}

// truncatedReviewJSON は出力上限で途中まで返った JSON。
// items が最後のフィールドなので外側の '{' と配列の '[' が閉じていない。
const truncatedReviewJSON = `{"summary":"確認しました","scores":{"specificity":4},"items":[{"quote":"売上を前年比120%に伸ばしました","message":"指摘1"`

// TestBuildRubricJSONHint はプロンプトの出力例が評価項目の定義から作られることを検証する。
// 出力例を手書きすると、項目を増減したときに検証と食い違う。
func TestBuildRubricJSONHint(t *testing.T) {
	hint := buildRubricJSONHint()
	for _, key := range ResumeRubricKeys() {
		if !strings.Contains(hint, `"`+key+`"`) {
			t.Errorf("出力例に %q が含まれない: %s", key, hint)
		}
	}
	if !strings.Contains(hint, "0-5") {
		t.Errorf("出力例に値域が含まれない: %s", hint)
	}
	if strings.Contains(hint, "score\"") {
		t.Errorf("総合スコアを出力させる例が残っている: %s", hint)
	}
}
