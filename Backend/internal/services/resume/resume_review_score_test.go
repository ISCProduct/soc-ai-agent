package resume

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
type aiStub struct {
	mu     sync.Mutex
	bodies []string
	calls  int
}

func (s *aiStub) next() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	body := s.bodies[min(s.calls, len(s.bodies)-1)]
	s.calls++
	return body
}

func (s *aiStub) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// newReviewService は OpenAI をスタブに向けた ResumeService を返す。
func newReviewService(t *testing.T, bodies ...string) (*ResumeService, *aiStub) {
	t.Helper()
	stub := &aiStub{bodies: bodies}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(stub.next()))
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
func reviewJSON(t *testing.T, scores map[string]int) string {
	t.Helper()
	payload := map[string]any{
		"summary": "確認しました",
		"items": []map[string]any{
			{"quote": "売上を前年比120%に伸ばしました", "message": "指摘1", "suggestion": "改善1", "severity": "info"},
			{"quote": "3名のチームでリーダーを担当しました", "message": "指摘2", "suggestion": "改善2", "severity": "warning"},
			{"quote": "基本情報技術者試験に合格しています", "message": "指摘3", "suggestion": "改善3", "severity": "critical"},
		},
	}
	if scores != nil {
		payload["scores"] = scores
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("レビューJSONの組み立てに失敗: %v", err)
	}
	return string(encoded)
}

func reviewBlocks() []models.ResumeTextBlock {
	texts := []string{
		"売上を前年比120%に伸ばしました",
		"3名のチームでリーダーを担当しました",
		"基本情報技術者試験に合格しています",
	}
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

// TestBuildReviewScoreItems_TruncatedOutputFails は出力が上限で切れたときに
// 切れたJSONを解析せず失敗させることを検証する（#1521 と同型の事故）。
//
// スタブが返すのは「items が1件・scores が欠けた」途中までのJSONで、
// decodeJSON の波括弧探索では読めてしまう形にしてある。
func TestBuildReviewScoreItems_TruncatedOutputFails(t *testing.T) {
	partial := `{"summary":"確認しました","scores":{"specificity":4},"items":[{"quote":"売上を前年比120%に伸ばしました","message":"指摘1"}]`
	svc, stub := newReviewService(t, responsesBody(t, partial, "max_output_tokens"))

	review, items, err := svc.buildReviewScoreItems(reviewBlocks(), "", "エンジニア", candidateTypeNewGrad, "")
	if err == nil {
		t.Fatalf("エラーが返るべき（review=%+v items=%d）", review, len(items))
	}
	if !strings.Contains(err.Error(), "切れ") {
		t.Errorf("エラーメッセージが上限到達を伝えていない: %v", err)
	}
	if review != nil || items != nil {
		t.Error("切れた出力から部分的な結果を作ってはいけない")
	}
	// 上限到達は再試行しても同じ位置で切れるので、やり直さない
	if got := stub.count(); got != 1 {
		t.Errorf("AI 呼び出し回数 = %d, want 1", got)
	}
}

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
