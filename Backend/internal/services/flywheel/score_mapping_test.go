package flywheel

import (
	"flag"
	"os"
	"strings"
	"testing"

	"Backend/domain/valueobject"
	"Backend/internal/models"
)

// TestResumeScoreMapping_TargetsAreCanonical は履歴書写像の反映先が正典であることを検証する（#929 / #1528）。
// 正典外だと user_weight_scores に書かれてもマッチング側から一度も引かれない死んだ行になる。
func TestResumeScoreMapping_TargetsAreCanonical(t *testing.T) {
	canonical := map[string]bool{}
	for _, c := range valueobject.AllWeightCategories() {
		canonical[string(c)] = true
	}
	if len(resumeScoreMapping) == 0 {
		t.Fatal("写像が空。テストが対象を見失っている")
	}
	for _, m := range resumeScoreMapping {
		if !canonical[m.category] {
			t.Errorf("正典外のカテゴリ %q に反映しようとしている", m.category)
		}
		if m.weight <= 0 || m.weight > 1 {
			t.Errorf("%s: weight %v は 0 < w <= 1 であるべき", m.category, m.weight)
		}
	}
	// 技術志向は #1528 で意図的に外した。根拠なく戻されないよう固定する。
	for _, m := range resumeScoreMapping {
		if m.category == string(valueobject.CategoryTechnical) {
			t.Error("技術志向は履歴書の完成度から導けないため反映対象外（#1528）")
		}
	}
}

// TestMapResumeScore_Monotonic は履歴書スコア全域が単調に反映されることを検証する。
// 旧実装は score>=70 でしか加点せず、69 と 0 が同じ（無変化）だった。
func TestMapResumeScore_Monotonic(t *testing.T) {
	scores := []int{0, 30, 50, 70, 100}
	prev := map[string]int{}

	for i, score := range scores {
		got := mapResumeScore(score, 0)
		if len(got) != len(resumeScoreMapping) {
			t.Fatalf("score=%d: 反映カテゴリ数 %d, want %d", score, len(got), len(resumeScoreMapping))
		}
		for _, cs := range got {
			if i > 0 && cs.score <= prev[cs.category] {
				t.Errorf("score=%d %s: %d は前段(%d)より大きくなるべき（単調増加していない）",
					score, cs.category, cs.score, prev[cs.category])
			}
			prev[cs.category] = cs.score
		}
	}

	// 中立点: スコア50は無情報なので中立50のまま
	for _, cs := range mapResumeScore(50, 0) {
		if cs.score != neutralScore {
			t.Errorf("%s: score=50 は中立 %d であるべき, got %d", cs.category, neutralScore, cs.score)
		}
	}
	// 低スコアは中立より下げる（旧実装では何も起きなかった）
	for _, cs := range mapResumeScore(20, 0) {
		if cs.score >= neutralScore {
			t.Errorf("%s: 低スコアは中立より下がるべき, got %d", cs.category, cs.score)
		}
	}
}

// TestMapResumeScore_CriticalPenalty は critical 件数がペナルティとして効き、
// 上限で飽和し、下限(0)を割らないことを検証する。
func TestMapResumeScore_CriticalPenalty(t *testing.T) {
	tests := []struct {
		name  string
		score int
		crits []int // 件数を増やしていく列
	}{
		{name: "高スコア", score: 90, crits: []int{0, 1, 2, 3, 4}},
		{name: "中スコア", score: 50, crits: []int{0, 1, 2, 3, 4}},
		{name: "低スコア", score: 5, crits: []int{0, 1, 2, 3, 4}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := map[string]int{}
			for i, crit := range tt.crits {
				for _, cs := range mapResumeScore(tt.score, crit) {
					if cs.score < 0 || cs.score > 100 {
						t.Fatalf("%s crit=%d: %d が 0〜100 の外", cs.category, crit, cs.score)
					}
					// 0 に張り付いている場合を除き、critical が増えれば下がる
					if i > 0 && cs.score > prev[cs.category] {
						t.Errorf("%s crit=%d: %d は前段(%d)より上がっている", cs.category, crit, cs.score, prev[cs.category])
					}
					prev[cs.category] = cs.score
				}
			}
			// 上限で飽和する（4件と20件で同じ）
			for i, cs := range mapResumeScore(tt.score, 4) {
				saturated := mapResumeScore(tt.score, 20)[i]
				if cs.score != saturated.score {
					t.Errorf("%s: ペナルティが上限(%d点)で飽和していない: 4件=%d, 20件=%d",
						cs.category, criticalPenaltyMax, cs.score, saturated.score)
				}
			}
		})
	}
}

// TestMapResumeScore_AlwaysInRange は極端な入力でも 0〜100 を渡すことを検証する。
// リポジトリ側の clamp に頼らず、呼び出し側でも範囲を守る（#1528）。
func TestMapResumeScore_AlwaysInRange(t *testing.T) {
	for _, score := range []int{-100, -1, 0, 1, 50, 99, 100, 101, 1000} {
		for _, crit := range []int{-5, 0, 1, 50} {
			for _, cs := range mapResumeScore(score, crit) {
				if cs.score < 0 || cs.score > 100 {
					t.Errorf("score=%d crit=%d %s: %d が 0〜100 の外", score, crit, cs.category, cs.score)
				}
			}
		}
	}
}

// TestMapInterviewScore_EvidenceChangesValue は同じルーブリック3でも
// evidence / 回答量の違いで別の値になることを検証する（旧実装は常に60固定）。
func TestMapInterviewScore_EvidenceChangesValue(t *testing.T) {
	thin := InterviewTranscriptStats{UserTurns: 1, AvgAnswerRunes: 5}
	rich := InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 150}

	// evidence 欠損は「記録が無い」＝中立扱いなので（#1554）、
	// ここでは evidence がある入力だけを並べて補正の効きを見る。
	tests := []struct {
		name     string
		evidence string
		stats    InterviewTranscriptStats
	}{
		{name: "evidence短い・発話わずか", evidence: strings.Repeat("あ", 20), stats: thin},
		{name: "evidence短い・発話十分", evidence: strings.Repeat("あ", 20), stats: rich},
		{name: "evidence中くらい・発話十分", evidence: strings.Repeat("あ", 60), stats: rich},
		{name: "evidence十分・発話十分", evidence: strings.Repeat("あ", 200), stats: rich},
	}

	seen := map[int]string{}
	prev := -1
	for _, tt := range tests {
		got := mapInterviewScore(3, tt.evidence, tt.stats)
		if other, dup := seen[got]; dup {
			t.Errorf("%s: %q と同じ値 %d になった。補正が効いていない", tt.name, other, got)
		}
		seen[got] = tt.name
		if got <= prev {
			t.Errorf("%s: %d は前段(%d)より大きくなるべき", tt.name, got, prev)
		}
		prev = got
		// ルーブリック1段の半分（±5点）に収まり、隣のバンドへ食い込まない
		if got < 3*rubricStep-interviewAdjustSpan/2 || got > 3*rubricStep+interviewAdjustSpan/2 {
			t.Errorf("%s: %d が rubric=3 の帯(%d±%d)を外れた", tt.name, got, 3*rubricStep, interviewAdjustSpan/2)
		}
	}
	if len(seen) != len(tests) {
		t.Fatalf("分解能が不足: %d 通りの入力で %d 通りの値しか出ていない", len(tests), len(seen))
	}
}

// TestMapInterviewScore_RubricIsPrimary はルーブリックが主軸であることを検証する。
// 補正でルーブリックの順序が入れ替わってはいけない（3が4より高く出ない）。
func TestMapInterviewScore_RubricIsPrimary(t *testing.T) {
	best := strings.Repeat("あ", 500)
	rich := InterviewTranscriptStats{UserTurns: 30, AvgAnswerRunes: 300}

	for rubric := range 5 {
		high := mapInterviewScore(rubric, best, rich)                      // 補正が最大に効いた下位
		low := mapInterviewScore(rubric+1, "", InterviewTranscriptStats{}) // 補正が最小の上位
		if high >= low {
			t.Errorf("rubric=%d(%d) が rubric=%d(%d) 以上になった。ルーブリックが主軸になっていない",
				rubric, high, rubric+1, low)
		}
	}

	// 値域は必ず 0〜100
	for rubric := -3; rubric <= 9; rubric++ {
		for _, ev := range []string{"", best} {
			for _, st := range []InterviewTranscriptStats{{}, rich, {UserTurns: -1, AvgAnswerRunes: -1}} {
				got := mapInterviewScore(rubric, ev, st)
				if got < 0 || got > 100 {
					t.Errorf("rubric=%d: %d が 0〜100 の外", rubric, got)
				}
			}
		}
	}
}

// TestNewInterviewTranscriptStats は発話ログから統計を作る部分を検証する。
func TestNewInterviewTranscriptStats(t *testing.T) {
	tests := []struct {
		name       string
		utterances []models.InterviewUtterance
		want       InterviewTranscriptStats
	}{
		{name: "発話なし", utterances: nil, want: InterviewTranscriptStats{}},
		{
			name: "AI発話は数えない",
			utterances: []models.InterviewUtterance{
				{Role: "ai", Text: "自己紹介をお願いします"},
				{Role: "user", Text: "あいうえお"},
			},
			want: InterviewTranscriptStats{UserTurns: 1, AvgAnswerRunes: 5},
		},
		{
			name: "空白だけの発話は数えない",
			utterances: []models.InterviewUtterance{
				{Role: "user", Text: "   "},
				{Role: "user", Text: "あいう"},
				{Role: "user", Text: "あいうえおか"},
			},
			want: InterviewTranscriptStats{UserTurns: 2, AvgAnswerRunes: 4},
		},
		{
			name:       "全部空白ならゼロ値",
			utterances: []models.InterviewUtterance{{Role: "user", Text: "\n\t "}},
			want:       InterviewTranscriptStats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewInterviewTranscriptStats(tt.utterances); got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestInterviewSignal_MissingStatsIsNeutral は発話統計が無い場合に
// 「話していない」ではなく「記録が無い（中立）」として扱うことを検証する。
func TestInterviewSignal_MissingStatsIsNeutral(t *testing.T) {
	// 統計なし: verbosity は 0.5 相当 → 最小の発話量(限りなく0)より高く出る
	none := mapInterviewScore(3, "", InterviewTranscriptStats{})
	worst := mapInterviewScore(3, "", InterviewTranscriptStats{UserTurns: 1, AvgAnswerRunes: 0})
	if none <= worst {
		t.Errorf("統計なし(%d)は最低の発話量(%d)より下げてはいけない", none, worst)
	}
	// 一方で、発話量が十分なケースより高くはならない
	rich := mapInterviewScore(3, "", InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 200})
	if none >= rich {
		t.Errorf("統計なし(%d)が発話十分(%d)以上になった", none, rich)
	}
}

// TestMapInterviewScore_ShortEvidenceIsBelowMissing は
// 「短い evidence は無記録より低く出る」という既知の歪みを固定する（#1554 / #1558 レビュー）。
//
// 欠損を中立(0.5)に置く以上この非単調性は消えない（判断の理由は interviewSignal のコメント）。
// 歪みの大きさをここで固定して、意図せず広がったら気付けるようにする。
// 上限は3点（rubric1段の 20点 に対して 15%）。これを超えたら中立値の置き方から見直す。
func TestMapInterviewScore_ShortEvidenceIsBelowMissing(t *testing.T) {
	rich := InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 120}

	tests := []struct {
		name          string
		evidenceRunes int
		want          int
	}{
		{name: "無記録（中立0.5）", evidenceRunes: 0, want: 62},
		{name: "1文字（最も不利）", evidenceRunes: 1, want: 59},
		{name: "中立の直前", evidenceRunes: 59, want: 62},
		{name: "中立と同じ（full の半分）", evidenceRunes: evidenceFullRunes / 2, want: 62},
		{name: "full で頭打ち", evidenceRunes: evidenceFullRunes, want: 65},
		{name: "full 超も頭打ちのまま", evidenceRunes: evidenceFullRunes * 2, want: 65},
	}

	missing := mapInterviewScore(3, "", rich)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapInterviewScore(3, strings.Repeat("あ", tt.evidenceRunes), rich)
			if got != tt.want {
				t.Errorf("evidence=%d文字: %d, want %d", tt.evidenceRunes, got, tt.want)
			}
			if loss := missing - got; loss > 3 {
				t.Errorf("evidence=%d文字: 無記録(%d)より %d 点低い。歪みが3点を超えた",
					tt.evidenceRunes, missing, loss)
			}
		})
	}
}

// TestMapInterviewScore_MissingEvidenceIsNeutral は evidence が無い場合に
// 減点せず中立に寄せることを検証する（#1554）。
//
// 修正前は evidence 欠損で signal の上限が 0.6*0+0.4*1 = 0.4 に下がり、
// どれだけ良い面接でも全カテゴリが必ずマイナス補正されていた。
// LLM が evidence を省く／evidence_json が壊れるだけで静かに全カテゴリが下がる経路だった。
func TestMapInterviewScore_MissingEvidenceIsNeutral(t *testing.T) {
	rich := InterviewTranscriptStats{UserTurns: 10, AvgAnswerRunes: 150}

	// evidence・発話統計の両方が欠損 = 補正なし。ルーブリックの ×20 がそのまま出る。
	for rubric := range 6 {
		want := rubric * rubricStep
		if got := mapInterviewScore(rubric, "", InterviewTranscriptStats{}); got != want {
			t.Errorf("rubric=%d: 記録が無いだけで %d 点になった（補正なしの %d 点であるべき）", rubric, got, want)
		}
	}

	// evidence だけが欠損しても、ルーブリックの帯から下へ外れない。
	if got := mapInterviewScore(3, "", rich); got < 3*rubricStep {
		t.Errorf("evidence欠損・発話十分で %d 点。%d 点を下回ってはいけない", got, 3*rubricStep)
	}
	// 一方で、evidence が十分にある場合より高くはならない。
	best := mapInterviewScore(3, strings.Repeat("あ", 200), rich)
	if got := mapInterviewScore(3, "", rich); got >= best {
		t.Errorf("evidence欠損(%d)が evidence十分(%d)以上になった", got, best)
	}
}

// TestBlendScore は移動平均が既存値を踏まえて動くことを検証する。
func TestBlendScore(t *testing.T) {
	tests := []struct {
		name     string
		existing int
		newValue int
		want     int
	}{
		{name: "同値なら動かない", existing: 60, newValue: 60, want: 60},
		{name: "既存70%+新30%", existing: 50, newValue: 100, want: 65}, // 35+30
		{name: "下向きにも効く", existing: 80, newValue: 0, want: 56},      // 56+0
		{name: "四捨五入", existing: 55, newValue: 70, want: 60},        // 38.5+21=59.5 → 60
		{name: "上限に張り付かない", existing: 100, newValue: 100, want: 100},
		{name: "下限に張り付かない", existing: 0, newValue: 0, want: 0},
		{name: "範囲外の入力も0〜100に収める", existing: 100, newValue: 1000, want: 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := blendScore(tt.existing, tt.newValue); got != tt.want {
				t.Errorf("blendScore(%d, %d) = %d, want %d", tt.existing, tt.newValue, got, tt.want)
			}
		})
	}
}

// updateGolden はドライランのゴールデンを更新するフラグ。
// 写像を意図して変えたときだけ使う:
//
//	cd Backend && go test ./internal/services/flywheel/ -run DryRun -update
var updateGolden = flag.Bool("update", false, "ドライランのゴールデンファイルを更新する")

const dryRunGoldenPath = "testdata/score_mapping_dryrun.golden"

// TestFormatScoreMappingDryRun は新旧比較表をゴールデンと突き合わせる（DB には触らない）。
//
// キャリブレーション定数（criticalPenaltyPerItem / criticalPenaltyMax / weight /
// evidenceFullRunes / answerFullRunes / answerFullTurns / evidenceSignalWeight など）は
// 単調性や飽和のアサーションでは固定できない。criticalPenaltyPerItem を 5→25 にしても
// 「critical が増えれば下がる」は成立してしまうため、出力そのものを固定する（#1555）。
//
// `go test ./internal/services/flywheel/ -run DryRun -v` で分布の変化を確認する。
func TestFormatScoreMappingDryRun(t *testing.T) {
	out := FormatScoreMappingDryRun()
	t.Log("\n" + out)

	if *updateGolden {
		if err := os.WriteFile(dryRunGoldenPath, []byte(out), 0o644); err != nil {
			t.Fatalf("ゴールデンの更新に失敗: %v", err)
		}
		t.Log("ゴールデンを更新した: " + dryRunGoldenPath)
		return
	}

	want, err := os.ReadFile(dryRunGoldenPath)
	if err != nil {
		t.Fatalf("ゴールデンが読めない: %v", err)
	}
	if out != string(want) {
		t.Errorf("写像の出力がゴールデンと一致しない。\n"+
			"キャリブレーションを意図して変えたなら -update で更新すること。\n"+
			"--- want ---\n%s\n--- got ---\n%s", want, out)
	}
}
