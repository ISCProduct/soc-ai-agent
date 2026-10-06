package aibench

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// stubTarget は LLM を呼ばずに決まった結果を返す評価対象。
type stubTarget struct {
	results []Observation
	calls   int
}

func (s *stubTarget) Name() string                   { return TargetResume }
func (s *stubTarget) Model() string                  { return "gpt-4o-mini" }
func (s *stubTarget) Endpoint() string               { return "stub" }
func (s *stubTarget) EstimateTokens(Case) (int, int) { return 1000, 500 }
func (s *stubTarget) Run(context.Context, Case) Observation {
	o := s.results[min(s.calls, len(s.results)-1)]
	s.calls++
	return o
}

func stubCases(ids ...string) []Case {
	out := make([]Case, 0, len(ids))
	for _, id := range ids {
		out = append(out, Case{ID: id, Target: TargetResume, Label: LabelGood, Input: Input{ResumeText: "本文"}})
	}
	return out
}

// 実行順が「ケースごとにn回連続」ではなく「1周ずつ」であることを固定する。
// 連続実行はプロンプトキャッシュに当たりやすく、レイテンシが実態より短く出る。
func TestRunは周回順に実行する(t *testing.T) {
	tgt := &stubTarget{results: []Observation{{Score: 0.5}}}
	obs, err := Run(context.Background(), tgt, stubCases("a", "b"), 2, nil)
	if err != nil {
		t.Fatalf("エラー: %v", err)
	}
	got := make([]string, 0, len(obs))
	for _, o := range obs {
		got = append(got, o.CaseID)
	}
	// CaseID は stub が返す Observation に入っていないので、Run が埋めた run 番号で見る
	wantRuns := []int{1, 1, 2, 2}
	for i, o := range obs {
		if o.Run != wantRuns[i] {
			t.Errorf("%d番目の run = %d, want %d（周回順になっていない）", i, o.Run, wantRuns[i])
		}
	}
	if len(obs) != 4 {
		t.Errorf("実行回数 = %d, want 4: %v", len(obs), got)
	}
}

// 設定の誤りを破損率として集計すると「破損率100%」というもっともらしい数字が出る。
// 検知したら打ち切ることを固定する。
func TestRunは設定エラーで打ち切る(t *testing.T) {
	tgt := &stubTarget{results: []Observation{
		{Score: 0.5},
		{Broken: true, BrokenReason: BrokenCallFailed, Fatal: true, Error: "Unauthorized"},
	}}
	obs, err := Run(context.Background(), tgt, stubCases("a", "b", "c"), 1, nil)
	if err == nil {
		t.Fatal("エラーになるべき")
	}
	if !strings.Contains(err.Error(), "設定の問題") {
		t.Errorf("エラー文 = %v", err)
	}
	// 打ち切ったので、成功した1件だけが残る（残り2件は実行されない）
	if len(obs) != 1 {
		t.Errorf("観測数 = %d, want 1", len(obs))
	}
	if tgt.calls != 2 {
		t.Errorf("呼び出し回数 = %d, want 2（3件目は呼ばない）", tgt.calls)
	}
}

// 認証エラーが Fatal として扱われることを、ES の評価関数側でも固定する。
func TestEvaluateESResponseは認証エラーをFatalにする(t *testing.T) {
	c := Case{ID: "es-001", Label: LabelGood, Input: Input{ESText: "本文"}}
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		got := evaluateESResponse(Observation{CaseID: c.ID}, c, status, []byte(`{"detail":"Unauthorized"}`))
		if !got.Fatal {
			t.Errorf("HTTP %d が Fatal になっていない", status)
		}
		if !strings.Contains(got.Error, "RAG_INTERNAL_TOKEN") {
			t.Errorf("HTTP %d のエラー文に対処法が無い: %q", status, got.Error)
		}
	}
}

func TestEstimateCost(t *testing.T) {
	tgt := &stubTarget{results: []Observation{{}}}
	// 2件 × 3回 = 6回、1回あたり 1000入力 + 500出力
	got := EstimateCost(tgt, stubCases("a", "b"), 3)
	want := 6000*0.15/1_000_000 + 3000*0.60/1_000_000
	if got < want-1e-12 || got > want+1e-12 {
		t.Errorf("概算コスト = %v, want %v", got, want)
	}
}
