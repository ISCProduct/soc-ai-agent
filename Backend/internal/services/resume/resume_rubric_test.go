package resume

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestResumeRubricWeights_Fixed は候補者区分ごとの重みを固定する（#1529）。
//
// 重みは総合スコアの意味そのものなので、値を動かすなら根拠（docs/wiki/scoring.md §2-5）
// と一緒に動かす。ここが落ちずに重みが変わることは無い。
func TestResumeRubricWeights_Fixed(t *testing.T) {
	tests := []struct {
		candidateType string
		want          map[string]int
	}{
		{
			candidateType: candidateTypeNewGrad,
			want: map[string]int{
				"specificity":  30,
				"achievement":  20,
				"role_fit":     15,
				"completeness": 20,
				"readability":  15,
			},
		},
		{
			candidateType: candidateTypeMidCareer,
			want: map[string]int{
				"specificity":  25,
				"achievement":  30,
				"role_fit":     25,
				"completeness": 10,
				"readability":  10,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.candidateType, func(t *testing.T) {
			got := resumeRubricWeights[tt.candidateType]
			if len(got) != len(tt.want) {
				t.Fatalf("項目数 = %d, want %d", len(got), len(tt.want))
			}
			sum := 0
			for key, want := range tt.want {
				if got[key] != want {
					t.Errorf("%s の重み = %d, want %d", key, got[key], want)
				}
				sum += got[key]
			}
			if sum != resumeRubricWeightTotal {
				t.Errorf("重みの合計 = %d, want %d", sum, resumeRubricWeightTotal)
			}
		})
	}
}

// TestResumeRubricWeights_CoverAllCriteria は全区分の重みが評価項目と1対1であることを検証する。
// 項目を増やして重みを足し忘れると、その項目は重み0で総合スコアに効かなくなる
// （LLM は出しているのに誰も読まない項目が生まれる）。
func TestResumeRubricWeights_CoverAllCriteria(t *testing.T) {
	keys := ResumeRubricKeys()
	if len(keys) == 0 {
		t.Fatal("評価項目が空。テストが対象を見失っている")
	}
	for candidateType, weights := range resumeRubricWeights {
		if len(weights) != len(keys) {
			t.Errorf("%s: 重みの項目数 = %d, want %d", candidateType, len(weights), len(keys))
		}
		for _, key := range keys {
			if weights[key] <= 0 {
				t.Errorf("%s: %s の重みが未定義または0", candidateType, key)
			}
			// 重みが満点(5)の倍数なら Σ(重み×項目スコア)/5 が常に整数になり、
			// ComputeResumeOverallScore の丸め方向が結果に影響しない。
			// この不変条件が崩れると 33/33/34 のような重みで60境界の値が
			// 丸めに依存し始めるので、そのときは丸め方向をテストで固定すること。
			if weights[key]%ResumeRubricScoreMax != 0 {
				t.Errorf("%s: %s の重み %d は満点 %d の倍数であるべき（丸めが効き始める）",
					candidateType, key, weights[key], ResumeRubricScoreMax)
			}
		}
	}
}

// TestComputeResumeOverallScore は項目スコアから総合スコアへの写像を固定する。
func TestComputeResumeOverallScore(t *testing.T) {
	tests := []struct {
		name          string
		candidateType string
		scores        map[string]int
		want          int
	}{
		{
			name:          "全項目5点は100点",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(5, 5, 5, 5, 5),
			want:          100,
		},
		{
			name:          "全項目3点は60点（基準を満たす標準的な書類）",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(3, 3, 3, 3, 3),
			want:          60,
		},
		{
			name:          "全項目0点は0点",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(0, 0, 0, 0, 0),
			want:          0,
		},
		{
			name:          "全項目1点は20点",
			candidateType: candidateTypeMidCareer,
			scores:        rubricScores(1, 1, 1, 1, 1),
			want:          20,
		},
		{
			// 30*5 + 20*4 + 15*4 + 20*5 + 15*4 = 450 → 450/5
			name:          "新卒・良い書類",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(5, 4, 4, 5, 4),
			want:          90,
		},
		{
			// 25*5 + 30*4 + 25*4 + 10*5 + 10*4 = 435 → 435/5
			name:          "中途・同じ項目スコアでも重みが違うので点が変わる",
			candidateType: candidateTypeMidCareer,
			scores:        rubricScores(5, 4, 4, 5, 4),
			want:          87,
		},
		{
			// 成果が高く網羅性が低い書類。新卒 30*3+20*5+15*3+20*1+15*3 = 300 → 60
			name:          "新卒・成果偏重の書類",
			candidateType: candidateTypeNewGrad,
			scores:        rubricScores(3, 5, 3, 1, 3),
			want:          60,
		},
		{
			// 中途 25*3+30*5+25*3+10*1+10*3 = 340 → 68。中途の方が高く出る
			name:          "中途・成果偏重の書類",
			candidateType: candidateTypeMidCareer,
			scores:        rubricScores(3, 5, 3, 1, 3),
			want:          68,
		},
		{
			name:          "未知の候補者区分は新卒の重みに倒す",
			candidateType: "unknown",
			scores:        rubricScores(3, 5, 3, 1, 3),
			want:          60,
		},
		{
			name:          "空の候補者区分も新卒の重み",
			candidateType: "",
			scores:        rubricScores(3, 5, 3, 1, 3),
			want:          60,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeResumeOverallScore(tt.scores, tt.candidateType)
			if err != nil {
				t.Fatalf("エラーが返った: %v", err)
			}
			if got != tt.want {
				t.Errorf("総合スコア = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestComputeResumeOverallScore_SeparatesQuality は良／中／悪の書類がスコアで分離することを
// 固定する（#1525 の評価ハーネスで実物を測る前の最低ライン）。
// 帯が重なると閾値（RESUME_COMPLETENESS_THRESHOLD 既定60）で区別できない。
func TestComputeResumeOverallScore_SeparatesQuality(t *testing.T) {
	tests := []struct {
		name   string
		scores map[string]int
		want   int
	}{
		{name: "良い書類", scores: rubricScores(5, 4, 4, 5, 4), want: 90},
		{name: "普通の書類", scores: rubricScores(3, 3, 3, 3, 3), want: 60},
		{name: "悪い書類", scores: rubricScores(1, 1, 0, 1, 1), want: 17},
	}

	prev := -1
	for _, tt := range tests {
		got, err := ComputeResumeOverallScore(tt.scores, candidateTypeNewGrad)
		if err != nil {
			t.Fatalf("%s: エラー: %v", tt.name, err)
		}
		if got != tt.want {
			t.Errorf("%s: 総合 = %d, want %d", tt.name, got, tt.want)
		}
		if prev >= 0 && got >= prev {
			t.Errorf("%s: %d は前段(%d)より低く出るべき", tt.name, got, prev)
		}
		prev = got
	}
}

// TestComputeResumeOverallScore_Monotonic は1項目だけ上げたら総合スコアも上がることを検証する。
// 重みが正で合計が固定なら成り立つ性質で、重みに負値や0が混ざると崩れる。
func TestComputeResumeOverallScore_Monotonic(t *testing.T) {
	for _, candidateType := range []string{candidateTypeNewGrad, candidateTypeMidCareer} {
		for _, key := range ResumeRubricKeys() {
			prev := -1
			for v := range ResumeRubricScoreMax + 1 {
				scores := rubricScores(2, 2, 2, 2, 2)
				scores[key] = v
				got, err := ComputeResumeOverallScore(scores, candidateType)
				if err != nil {
					t.Fatalf("%s/%s=%d: エラー: %v", candidateType, key, v, err)
				}
				if got <= prev {
					t.Errorf("%s/%s=%d: 総合 %d は前段(%d)より大きくなるべき", candidateType, key, v, got, prev)
				}
				prev = got
			}
		}
	}
}

// TestValidateResumeRubricScores は検証が弾くものを固定する。
// ここを通ったスコアだけが user_weight_scores へ流れる（docs/wiki/scoring.md §2-3）。
func TestValidateResumeRubricScores(t *testing.T) {
	tests := []struct {
		name       string
		scores     map[string]int
		wantErr    bool
		wantErrHas string
	}{
		{name: "正常", scores: rubricScores(0, 1, 2, 3, 4), wantErr: false},
		{name: "上限", scores: rubricScores(5, 5, 5, 5, 5), wantErr: false},
		{name: "nil", scores: nil, wantErr: true, wantErrHas: "空"},
		{name: "空", scores: map[string]int{}, wantErr: true, wantErrHas: "空"},
		{
			name:       "項目欠落",
			scores:     map[string]int{"specificity": 3, "achievement": 3, "role_fit": 3, "completeness": 3},
			wantErr:    true,
			wantErrHas: "readability",
		},
		{
			name: "未知の項目",
			scores: map[string]int{
				"specificity": 3, "achievement": 3, "role_fit": 3, "completeness": 3, "readability": 3,
				"passion": 5,
			},
			wantErr:    true,
			wantErrHas: "passion",
		},
		{
			name:       "100点満点で返された",
			scores:     rubricScores(80, 90, 70, 60, 100),
			wantErr:    true,
			wantErrHas: "範囲外",
		},
		{
			name:       "負値",
			scores:     rubricScores(-1, 3, 3, 3, 3),
			wantErr:    true,
			wantErrHas: "範囲外",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateResumeRubricScores(tt.scores)
			if tt.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if err != nil && tt.wantErrHas != "" && !strings.Contains(err.Error(), tt.wantErrHas) {
				t.Errorf("エラーに %q が含まれない: %v", tt.wantErrHas, err)
			}
			// 不正なスコアからは総合スコアを作らない（スコア無しとして扱わせる）
			if _, scoreErr := ComputeResumeOverallScore(tt.scores, candidateTypeNewGrad); (scoreErr != nil) != tt.wantErr {
				t.Errorf("ComputeResumeOverallScore のエラー = %v, wantErr = %v", scoreErr, tt.wantErr)
			}
		})
	}
}

// TestResumeRubricLevels_Defined は全項目に 0〜5 のレベル定義があることを固定する（#1584）。
//
// レベル定義が無いと「3点が2点や4点と何が違うか」がプロンプトに無く、
// good と mid が同じ点に潰れる（Issue #1584 の実測）。項目を足してレベルを
// 書き忘れると空文字のレベルがプロンプトに出るので、ここで止める。
// **配列型はこれを止めない**（短いコンポジットリテラルはゼロ値で埋まる）。
//
// 添字と本文の対応（順序）はここでは見ていない。golden が見る
// （TestResumeRubricPromptSection_Golden）。
func TestResumeRubricLevels_Defined(t *testing.T) {
	for _, c := range ResumeRubricCriteria() {
		t.Run(c.Key, func(t *testing.T) {
			seen := map[string]int{}
			for score, level := range c.Levels {
				if strings.TrimSpace(level) == "" {
					t.Errorf("%d点のレベル定義が空", score)
					continue
				}
				if prev, dup := seen[level]; dup {
					t.Errorf("%d点と%d点のレベル定義が同一（隣接レベルを判別できない）: %q", prev, score, level)
				}
				seen[level] = score
			}
		})
	}
}

// TestResumeRubricPromptSection_Golden はプロンプトの評価基準セクションを1文字も違わず固定する（#1584）。
//
// **これが無いとレベル定義を逆順（0点＝最良）にしても全テストが通る。**
// 他のテストは期待値を c.Levels 自身から取って部分一致を見るだけなので、
// 添字と本文の対応を誰も見ていなかった。採点が反転したルーブリックが
// user_weight_scores まで流れるので、順序は文字列そのもので固定する。
//
// レベル定義を意図して変えたときは、失敗出力に出る実際のセクションを
// testdata/rubric_prompt.golden へ丸ごと置き換え、**弁別力をハーネスで
// 測り直してから**コミットすること（docs/wiki/scoring.md §2-5）。
func TestResumeRubricPromptSection_Golden(t *testing.T) {
	const goldenPath = "testdata/rubric_prompt.golden"
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("golden の読み込みに失敗: %v", err)
	}
	got := BuildResumeRubricPromptSection()
	if got != string(want) {
		t.Errorf("プロンプトの評価基準セクションが golden と一致しない。\n"+
			"意図した変更なら %s を下記の内容へ置き換え、ハーネスで弁別力を測り直すこと。\n"+
			"--- 実際の出力 ---\n%s\n--- golden ---\n%s", goldenPath, got, want)
	}
}

// TestBuildResumeRubricPromptSection はプロンプトが評価項目の定義だけから作られることを検証する。
// プロンプトに項目をハードコードすると、項目を増減したときに検証と食い違う。
func TestBuildResumeRubricPromptSection(t *testing.T) {
	section := BuildResumeRubricPromptSection()
	for _, c := range ResumeRubricCriteria() {
		for _, want := range []string{c.Key, c.Label, c.Description} {
			if !strings.Contains(section, want) {
				t.Errorf("プロンプトに %q が含まれない:\n%s", want, section)
			}
		}
		// レベル定義（アンカー）が全段載っていること（#1584）。
		// ここが漏れるとプロンプトは Yes/No 型の説明だけに戻る。
		for score, level := range c.Levels {
			if !strings.Contains(section, level) {
				t.Errorf("%s の%d点のレベル定義がプロンプトに含まれない: %q", c.Key, score, level)
			}
			if !strings.Contains(section, fmt.Sprintf("%d点:", score)) {
				t.Errorf("%d点の見出しがプロンプトに無い:\n%s", score, section)
			}
		}
	}
	if !strings.Contains(section, "0〜5") {
		t.Errorf("値域が書かれていない:\n%s", section)
	}
	// 重みを載せると LLM が総合点を逆算するため、入れない
	for _, weight := range []string{"30", "25"} {
		if strings.Contains(section, weight) {
			t.Errorf("重み %q がプロンプトに漏れている:\n%s", weight, section)
		}
	}
}

// rubricScores は定義順（specificity, achievement, role_fit, completeness, readability）で
// 項目スコアを組み立てる。
func rubricScores(specificity, achievement, roleFit, completeness, readability int) map[string]int {
	return map[string]int{
		"specificity":  specificity,
		"achievement":  achievement,
		"role_fit":     roleFit,
		"completeness": completeness,
		"readability":  readability,
	}
}
