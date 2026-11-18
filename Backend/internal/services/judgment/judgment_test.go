package judgment

import "testing"

// 合成のプロファイル。実データは使わない。
func scores(vals ...int) map[string]int {
	cats := []string{"c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "c10"}
	m := map[string]int{}
	for i, v := range vals {
		m[cats[i]] = v
	}
	return m
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name      string
		in        Input
		wantReady bool
		wantPct   int
	}{
		{"何も無い", Input{}, false, 0},
		{"全部そろう（6問・6カテゴリ・差20）は、完了", Input{ValidAnswers: 6, Scores: scores(70, 50, 40, 50, 60, 55)}, true, 100},
		{"15問に満たなくても、判定できたら完了", Input{ValidAnswers: 8, Scores: scores(80, 30, 55, 60, 45, 70)}, true, 100},
		{"床（6問）に満たなければ、他が満点でも完了にしない", Input{ValidAnswers: 5, Scores: scores(80, 30, 55, 60, 45, 70)}, false, 94},
		{"職種の決定は条件にしない（まだ決めていない人も、傾向が出れば完了）", Input{ValidAnswers: 8, Scores: scores(80, 30, 55, 60, 45, 70)}, true, 100},
		{"評価済みが下限（6）未満なら、続ける。推薦の暫定表示の下限（4）に届いていても完了にしない", Input{ValidAnswers: 10, Scores: scores(80, 30, 55, 60)}, false, 88},
		{"点が横並び（差が小さい）なら、20問答えても続ける", Input{ValidAnswers: 20, Scores: scores(50, 52, 51, 53, 50, 49)}, false, 73},
		{"0点は未評価として数える", Input{ValidAnswers: 10, Scores: scores(80, 0, 0, 0, 30)}, false, 77},
		{"15問を超えても、判定できなければ続ける", Input{ValidAnswers: 18, Scores: scores(50, 50, 50, 50)}, false, 55},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Evaluate(tt.in)
			if got.Ready != tt.wantReady {
				t.Fatalf("Ready=%v, want %v（%+v）", got.Ready, tt.wantReady, got)
			}
			if got.Percent != tt.wantPct {
				t.Fatalf("Percent=%d, want %d（%+v）", got.Percent, tt.wantPct, got)
			}
		})
	}
}

// 100% は、判定できたときだけ。判定できていないのに 100% と見せない。
func TestEvaluate_PercentIs100OnlyWhenReady(t *testing.T) {
	for v := 0; v <= 40; v += 5 {
		for sp := 0; sp <= 40; sp += 10 {
			got := Evaluate(Input{ValidAnswers: v, Scores: scores(50, 50+sp, 50, 50, 50, 50)})
			if (got.Percent == 100) != got.Ready {
				t.Fatalf("Percent=%d, Ready=%v（%+v）", got.Percent, got.Ready, got)
			}
			if got.Percent < 0 || got.Percent > 100 {
				t.Fatalf("範囲外: %d", got.Percent)
			}
		}
	}
}

// 回答が増えて、スコアが広がるほど、進み具合は下がらない（同じ職種・同じ点の分布で、回答数だけが増える場合）。
func TestEvaluate_MonotonicInAnswers(t *testing.T) {
	prev := -1
	for v := 0; v <= 12; v++ {
		got := Evaluate(Input{ValidAnswers: v, Scores: scores(70, 40, 55, 60)})
		if got.Percent < prev {
			t.Fatalf("回答%dで下がった: %d < %d", v, got.Percent, prev)
		}
		prev = got.Percent
	}
}

func TestSuggestProvisional(t *testing.T) {
	tests := []struct {
		answered int
		ready    bool
		want     bool
	}{
		{SoftCapAnswers - 1, false, false},
		{SoftCapAnswers, false, true},
		{SoftCapAnswers + 5, false, true},
		{SoftCapAnswers + 5, true, false}, // 判定できていれば、暫定の導線は要らない
	}
	for _, tt := range tests {
		if got := SuggestProvisional(tt.answered, tt.ready); got != tt.want {
			t.Errorf("SuggestProvisional(%d, %v)=%v, want %v", tt.answered, tt.ready, got, tt.want)
		}
	}
}
