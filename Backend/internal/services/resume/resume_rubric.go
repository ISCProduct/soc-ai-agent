package resume

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// 履歴書レビューのルーブリック（#1529）。
//
// 評価項目・採点基準・値域・重みはここだけで定義し、プロンプトと検証の両方が
// これを読む。面接側（interview/interview_rubric.go）と同じ構造である。
// プロンプト側と検証側に別々に書くと、片方だけ増減したときに
// 「LLM は出しているのに検証が知らない項目」が静かに生まれる。
//
// 総合スコア（0〜100）は項目スコアの加重和から**サーバー側で**算出する。
// LLM に総合点を直接言わせない。以前は `{"score":0-100}` と書かせるだけで
// 採点基準が無く、さらに `score <= 0` なら70・AI失敗時も70だったため、
// 「70点」が良かったから70なのか失敗したから70なのか区別できなかった。

// ResumeRubricCriterion は評価項目1つ。
type ResumeRubricCriterion struct {
	// Key は LLM 出力 JSON のキー。item_scores_json へもこのキーで保存される。
	Key string
	// Label は日本語の項目名。
	Label string
	// Description は採点基準。プロンプトへそのまま入る。
	Description string
	// Levels は 0〜5 のレベル定義（アンカー）。添字がスコアそのもの。
	// プロンプトへそのまま入る（#1584）。
	Levels ResumeRubricLevels
}

// ResumeRubricScoreMin / ResumeRubricScoreMax は項目スコアの値域。
// この範囲外は LLM の出力ミスとして弾く（100点満点で返す事故が実際に起こりうる）。
const (
	ResumeRubricScoreMin = 0
	ResumeRubricScoreMax = 5
)

// ResumeRubricLevels は1項目のレベル定義。添字 0〜ResumeRubricScoreMax がスコア。
// 配列にしてあるので、値域を広げたらレベルを書き足さないとコンパイルが通らない。
type ResumeRubricLevels [ResumeRubricScoreMax + 1]string

// resumeRubricCriteria は履歴書の評価項目。順序はプロンプトの記載順で、
// 画面の内訳表示もこの順に合わせる（frontend/app/resume/utils.ts の RUBRIC_CRITERIA）。
//
// レベル定義（Levels）は「何があれば何点か」を数えられる形で書く（#1584）。
// 「優れている」「普通」のような主観語だけにすると、Description の Yes/No 型と
// 同じで 3 が 2 や 4 と何が違うかをモデルに伝えられない。
// 上のレベルは下のレベルを満たした上での積み増しとして書き、
// 隣り合うレベルの差が本文を見れば判定できることを条件にしている。
var resumeRubricCriteria = []ResumeRubricCriterion{
	{"specificity", "具体性", "数値・固有名詞・具体的な行動が含まれているか", ResumeRubricLevels{
		"具体的な記述が無く、「頑張りました」「コミュニケーション能力があります」のような形容だけ",
		"活動名・所属・肩書きだけがあり、自分が何をしたかの行動が書かれていない",
		"自分の行動は書かれているが、数値・固有名詞・期間がいずれも無い",
		"自分の行動に加えて、数値・固有名詞・期間のいずれか1種類がある",
		"数値・固有名詞・期間のうち2種類以上があり、主要な活動の規模と自分の役割が読み取れる",
		"主要な活動すべてに数値と固有名詞があり、何をどう変えたかの手順まで追える",
	}},
	{"achievement", "成果の明示", "取組の結果が読み取れるか", ResumeRubricLevels{
		"取組の結果に触れた記述が無い",
		"「貢献できた」「評価された」のように、結果を主観の言葉だけで述べている",
		"結果は書かれているが、どの取組による結果なのか対応が読み取れない",
		"取組と結果が対応しており、結果が定性的に読み取れる",
		"結果が件数・時間・金額・人数などの数値で示されている",
		"結果が数値で示され、かつ変化（前→後）または再現性（継続・横展開・他者への移管）まで書かれている",
	}},
	{"role_fit", "職種適合", "応募職種の評価軸に対応する記述があるか", ResumeRubricLevels{
		"応募職種に関係する記述が無い",
		"職種名や志望の表明だけで、裏付ける経験・学習が無い",
		"関連しそうな経験はあるが、応募職種の評価軸との対応が書かれていない",
		"応募職種の評価軸に対応する経験・学習が1つ書かれている",
		"対応する経験・学習が2つ以上あり、職種で使う技能が具体名（言語・ツール・資格・業務名）で書かれている",
		"さらに、その経験を応募先の業務でどう使うかが志望動機と結びついている",
	}},
	{"completeness", "記載の網羅性", "必要項目が埋まっているか", ResumeRubricLevels{
		"氏名・学歴を含む基本項目がほとんど埋まっていない",
		"氏名・学歴だけで、職歴・資格・自己PR・志望動機のいずれも無い",
		"必要項目の半分以上が空欄、または学歴・職歴の年月が欠けている",
		"必要項目はすべて埋まっているが、期間・役割などの粒度が一部欠けている",
		"必要項目がすべて埋まり、学歴・職歴に年月があって欠落が無い",
		"さらに、在籍期間と資格の取得時期に矛盾が無く、応募職種の判断に必要な情報が補われている",
	}},
	{"readability", "読みやすさ", "一文の長さ・表記統一・構成", ResumeRubricLevels{
		"文として成立しておらず意味が取れない",
		"一文が極端に長い、または箇条書きと文章が混在して読み進められない",
		"意味は取れるが、一文が長すぎる箇所や表記の不統一（西暦と和暦の混在など）が目立つ",
		"一文はおおむね60文字以内で、項目ごとに整理されている",
		"さらに年号・数字・文体が統一され、段落の切り方が内容の区切りと一致している",
		"さらに結論→根拠→補足の順で構成され、読み手が探さずに要点へ到達できる",
	}},
}

// 候補者区分。フロント（ResumeReviewForm）の選択肢と同じ値。
const (
	candidateTypeNewGrad   = "new_grad"
	candidateTypeMidCareer = "mid_career"
)

// resumeRubricWeights は候補者区分ごとの項目の重み。各区分で合計100。
//
// 新卒と中途で見るものが違う。新卒は職務上の成果そのものが乏しいのが前提で、
// 書ける範囲をどれだけ具体的に・漏れなく書けているかが評価対象になる。
// 中途は成果と職種適合が選考の中心で、書式の網羅性は相対的に軽い。
//
// 値は現場の選考基準ではなく上記の方針からの仮置きである。#1525 の評価ハーネスで
// 良／中／悪の分離を測ったうえで見直すこと（docs/wiki/scoring.md §2-5）。
var resumeRubricWeights = map[string]map[string]int{
	candidateTypeNewGrad: {
		"specificity":  30, // 新卒は具体性が唯一の差
		"achievement":  20, // 職務成果は期待しにくいので抑える
		"role_fit":     15, // 職種経験が無い前提なので弱く見る
		"completeness": 20, // 書式の抜けは新卒で頻発する
		"readability":  15,
	},
	candidateTypeMidCareer: {
		"specificity":  25,
		"achievement":  30, // 中途は成果が選考の中心
		"role_fit":     25, // 職種の経験一致が最重要に近い
		"completeness": 10, // 書式の抜けより中身
		"readability":  10,
	},
}

// resumeRubricWeightTotal は重みの合計（＝100%）。
const resumeRubricWeightTotal = 100

// ResumeRubricCriteria は評価項目の一覧を返す。
func ResumeRubricCriteria() []ResumeRubricCriterion {
	out := make([]ResumeRubricCriterion, len(resumeRubricCriteria))
	copy(out, resumeRubricCriteria)
	return out
}

// ResumeRubricKeys は評価項目のキー一覧を定義順で返す。
func ResumeRubricKeys() []string {
	keys := make([]string, len(resumeRubricCriteria))
	for i, c := range resumeRubricCriteria {
		keys[i] = c.Key
	}
	return keys
}

// resumeWeightsFor は候補者区分に対応する重みを返す。
// 未知・空の区分は新卒として扱う（フロントの既定値が new_grad）。
func resumeWeightsFor(candidateType string) map[string]int {
	if w, ok := resumeRubricWeights[strings.TrimSpace(candidateType)]; ok {
		return w
	}
	return resumeRubricWeights[candidateTypeNewGrad]
}

// BuildResumeRubricPromptSection は評価基準の説明をプロンプト用に組み立てる。
//
// 各項目のレベル定義（アンカー）を全段載せる（#1584）。載せないと
// 「3点が2点や4点と何が違うか」がモデル側に無く、good と mid が同じ点に潰れる。
//
// 重みは載せない。載せると LLM が総合点を逆算して書き始め、
// サーバー側で算出する意味が無くなる。
func BuildResumeRubricPromptSection() string {
	lines := make([]string, 0, len(resumeRubricCriteria)*(ResumeRubricScoreMax+2)+2)
	lines = append(lines, fmt.Sprintf("## 評価基準（各スコアは%d〜%dの整数）", ResumeRubricScoreMin, ResumeRubricScoreMax))
	lines = append(lines, "各項目は下のレベル定義に照らし、本文が満たしている最も高いレベルの数字を選んでください。印象で決めないこと。")
	for _, c := range resumeRubricCriteria {
		lines = append(lines, fmt.Sprintf("- %s（%s）: %s", c.Key, c.Label, c.Description))
		for score, level := range c.Levels {
			lines = append(lines, fmt.Sprintf("  - %d点: %s", score, level))
		}
	}
	return strings.Join(lines, "\n")
}

// ValidateResumeRubricScores は LLM が返した項目スコアがルーブリックに従うかを検証する。
//
// 弾く理由は、誤ったスコアが総合スコアを経由して user_weight_scores へ流れると
// マッチングと教員向け傾向分析の両方に静かに混ざり、後から区別できないため。
// 欠損は画面に出るが、誤りは出ない（docs/wiki/scoring.md §2-3）。
func ValidateResumeRubricScores(scores map[string]int) error {
	if len(scores) == 0 {
		return fmt.Errorf("スコアが空")
	}

	var missing []string
	for _, key := range ResumeRubricKeys() {
		if _, ok := scores[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("必須の評価項目が欠けている: %s", strings.Join(missing, ", "))
	}

	known := map[string]bool{}
	for _, key := range ResumeRubricKeys() {
		known[key] = true
	}
	var unknown []string
	for key := range scores {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		// map の順序は不定なので、エラー文を安定させる
		sort.Strings(unknown)
		return fmt.Errorf("未知の評価項目が含まれる: %s", strings.Join(unknown, ", "))
	}

	var outOfRange []string
	for _, key := range ResumeRubricKeys() {
		v := scores[key]
		if v < ResumeRubricScoreMin || v > ResumeRubricScoreMax {
			outOfRange = append(outOfRange, fmt.Sprintf("%s=%d", key, v))
		}
	}
	if len(outOfRange) > 0 {
		return fmt.Errorf("スコアが %d〜%d の範囲外: %s",
			ResumeRubricScoreMin, ResumeRubricScoreMax, strings.Join(outOfRange, ", "))
	}
	return nil
}

// ComputeResumeOverallScore は項目スコアから総合スコア（0〜100）を算出する。
//
//	総合 = Σ(重み × 項目スコア) / (重み合計 × 5) × 100
//
// 線形写像なので全項目3点は60点、全項目5点で100点、全項目0点で0点になる。
// 曲線を入れないのは、点の意味を「各項目の平均を100点満点に直したもの」から
// 動かさないため。
//
// 60は RESUME_COMPLETENESS_THRESHOLD の既定値と一致する。**一致そのものは算術の
// 結果**（線形写像で 3/5 が 60%）だが、「3点」の中身は #1584 でレベル定義を入れた
// ぶん決まっている。全項目3点は「必要項目は埋まり、行動と結果は対応しているが、
// 数値の裏付けが1種類しか無い書類」であり、これが要対応でない側の下限にあたる。
// 境界値そのものの実データ校正（good/mid の平均がどこに来るか）は #1525 のハーネス。
//
// 重みが ResumeRubricScoreMax の倍数である限り weighted/5 は整数になり、
// 丸め方向は結果に影響しない（TestResumeRubricWeights_CoverAllCriteria が固定）。
//
// ルーブリック違反のときはエラーを返し、スコアを算出しない。
// 呼び出し側はスコア無しとして扱う（固定値を入れない）。
func ComputeResumeOverallScore(scores map[string]int, candidateType string) (int, error) {
	if err := ValidateResumeRubricScores(scores); err != nil {
		return 0, err
	}
	weights := resumeWeightsFor(candidateType)

	weighted, total := 0, 0
	for _, key := range ResumeRubricKeys() {
		weighted += weights[key] * scores[key]
		total += weights[key]
	}
	if total <= 0 {
		// 重みの定義ミス。0除算で NaN を保存するより明示的に落とす。
		return 0, fmt.Errorf("候補者区分 %q の重み合計が0", candidateType)
	}
	return int(math.Round(float64(weighted) * 100 / float64(total*ResumeRubricScoreMax))), nil
}
