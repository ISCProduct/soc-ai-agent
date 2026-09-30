package interview

import (
	"fmt"
	"sort"
	"strings"

	"Backend/internal/models"
	"Backend/internal/services/shared/textsim"
)

// 構造化面接ルーブリック（#795）。
//
// 評価項目はここだけで定義し、プロンプトと検証の両方がこれを読む。
// プロンプト側と検証側に別々に書くと、片方だけ増減したときに
// 「LLM は出しているのに検証が知らない項目」が生まれ、静かに欠落する。
//
// user_weight_scores の10カテゴリとは別物である。こちらは面接1回の採点軸で、
// flywheel の interviewScoreMapping が10カテゴリへ変換する。二重定義ではない。

// RubricCriterion は評価項目1つ。
type RubricCriterion struct {
	// Key は LLM 出力 JSON のキー。DB へもこのキーで保存される。
	Key string
	// Label は日本語の項目名。
	Label string
	// Description は採点基準。プロンプトへそのまま入る。
	Description string
}

// RubricScoreMin / RubricScoreMax はスコアの値域。
// この範囲外は LLM の出力ミスとして弾く（#795）。
const (
	RubricScoreMin = 0
	RubricScoreMax = 5
)

// reportGenerationAttempts はレポート生成の試行回数（初回＋やり直し）。
// スキーマ違反は温度0.4のばらつきによることが多く、1度やり直せば大抵通る。
// 増やしても費用が線形に増えるだけなので2に留める。
const reportGenerationAttempts = 2

// rubricCriteria は面接の評価項目。順序はプロンプトの記載順。
var rubricCriteria = []RubricCriterion{
	{"logic", "論理性", "回答が筋道立っているか、主張に一貫性があるか"},
	{"specificity", "具体性", "具体的なエピソードや数値が含まれているか"},
	{"ownership", "主体性", "「私が〜した」という自分起点の表現があるか"},
	{"communication", "コミュニケーション力", "簡潔・明確に伝えられているか、聞き返しが少ないか"},
	{"enthusiasm", "積極性・熱意", "志望動機や意欲が伝わっているか。取り組みのきっかけや継続の姿勢も評価材料に含めてよい"},
}

// RubricCriteria は評価項目の一覧を返す。
func RubricCriteria() []RubricCriterion {
	out := make([]RubricCriterion, len(rubricCriteria))
	copy(out, rubricCriteria)
	return out
}

// RubricKeys は評価項目のキー一覧を定義順で返す。
func RubricKeys() []string {
	keys := make([]string, len(rubricCriteria))
	for i, c := range rubricCriteria {
		keys[i] = c.Key
	}
	return keys
}

// BuildRubricPromptSection は評価基準の説明をプロンプト用に組み立てる。
func BuildRubricPromptSection() string {
	lines := make([]string, 0, len(rubricCriteria)+1)
	lines = append(lines, fmt.Sprintf("## 評価基準（各スコアは%d〜%dの整数）", RubricScoreMin, RubricScoreMax))
	for _, c := range rubricCriteria {
		lines = append(lines, fmt.Sprintf("- %s（%s）: %s", c.Key, c.Label, c.Description))
	}
	return strings.Join(lines, "\n")
}

// ValidateRubricScores は LLM が返したスコアがルーブリックに従うかを検証する（#795）。
//
// 弾く理由は、誤ったスコアが user_weight_scores へ流れると
// マッチングと教員向け傾向分析の両方に静かに混ざり、後から区別できないため。
// 欠損は画面に出るが、誤りは出ない（docs/wiki/scoring.md §2-3）。
func ValidateRubricScores(scores map[string]int) error {
	if len(scores) == 0 {
		return fmt.Errorf("スコアが空")
	}

	var missing []string
	for _, key := range RubricKeys() {
		if _, ok := scores[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("必須の評価項目が欠けている: %s", strings.Join(missing, ", "))
	}

	known := map[string]bool{}
	for _, key := range RubricKeys() {
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
	for _, key := range RubricKeys() {
		v := scores[key]
		if v < RubricScoreMin || v > RubricScoreMax {
			outOfRange = append(outOfRange, fmt.Sprintf("%s=%d", key, v))
		}
	}
	if len(outOfRange) > 0 {
		return fmt.Errorf("スコアが %d〜%d の範囲外: %s",
			RubricScoreMin, RubricScoreMax, strings.Join(outOfRange, ", "))
	}
	return nil
}

// EvidenceMatchThreshold は evidence を「実発話に基づく」と認める最小一致度（#1527 / #1566）。
//
// 照合は**内容語だけ**で行う（textsim.ContentBestMatch）。実測した分布は下表で、
// 全文の文字bigramで測っていた #1527 時点の値とは別物になっている
// （例と測り方は docs/wiki/scoring.md §2-4）。
//
//	引用そのまま / 表記ゆれ / 助詞違い  1.00        照合
//	要約された引用                     0.29〜1.00  照合
//	実引用＋事実の継ぎ足し             0.29〜0.75  照合（通ってしまう）
//	---------------- しきい値 0.25 ----------------
//	捏造（内容語を一部流用）           0.20〜0.22  未照合
//	捏造（発話の述語末尾だけを流用）   0.00〜0.14  未照合
//	捏造（述語も流用しない）           0.00〜0.13  未照合
//	フィラー・相槌（「はい」等）       0.00        未照合（内容語が残らない）
//	記号・絵文字だけ                   0.00        未照合
//
// 使える帯は 0.143〜0.286（実測）。下限は内容語を流用しない捏造の最高値、
// 上限は正当な要約の最低値。TestEvidenceMatchThreshold_Boundary が上下から固定している。
//
// **通った根拠が実発話に基づくとは限らない。** 内容語を発話から流用して事実だけ
// 入れ替えた捏造（「部費を3倍にすることができました」）は正当な要約と同じ帯に入り、
// しきい値では分離できない（`TestValidateEvidence_KnownLimitation`、Issue #1566）。
//
// 評価項目やプロンプトを変えたときは textsim で実測してこの表を引き直すこと。
const EvidenceMatchThreshold = 0.25

// EvidenceCheck は evidence の照合結果。
type EvidenceCheck struct {
	// Checked は照合対象にした項目数。空文字の項目は数えない。
	Checked int
	// Unmatched は照合できなかったキー（昇順）。
	Unmatched []string
}

// Matched は照合できた項目数を返す。
func (c EvidenceCheck) Matched() int { return c.Checked - len(c.Unmatched) }

// SpokenContent は受験者(role=user)の発話を内容語だけに落として返す（#1527 / #1566）。
//
// 面接官(role=ai)の発話を混ぜると、質問文をそのまま根拠として引用しても
// 照合が通ってしまう。照合したいのは「学生が言ったか」である。
//
// 内容語だけにするのは、述語末尾や相槌の流用を弾くため（textsim.ContentRunes）。
// 戻り値は ValidateEvidence（＝textsim.ContentBestMatch）専用で、
// 助詞を含む生の needle と突き合わせても意味のある値にはならない。
//
// evidence の件数ぶん作り直さないよう、呼び出し側で1度だけ作って使い回す。
func SpokenContent(utterances []models.InterviewUtterance) textsim.Bigrams {
	var b strings.Builder
	for _, u := range utterances {
		if u.Role != "user" {
			continue
		}
		b.WriteString(u.Text)
		b.WriteString("\n")
	}
	return textsim.NewContent(b.String())
}

// ValidateEvidence は evidence の各項目が実際の受験者発話に基づくかを照合する（#1527）。
//
// LLM は根拠として「言っていない発言」を書きうる。スコアの値域と違って
// evidence は自由記述なのでスキーマ検証では捕まらないが、学生向け・教員向けの
// 両レポートに表示される。教員がレポートを前提に指導する運用では、
// 根拠の捏造はスコアの誤りより直接に信頼を損なう。
//
// 照合はローカル計算だけで行う（LLM を再度呼ばない）。内容語の文字bigramの
// Dice 係数なので形態素解析器も要らず、言い直し・助詞の差・表記ゆれは自然に吸収する。
//
// 空文字の項目は照合対象にしない（Checked にも数えない）。「根拠が無い」ことは
// 既に欠落として表現されており、捏造ではないため再生成を促す必要がない。
// 一方で記号・絵文字・相槌だけの項目は空ではないので照合対象になり、
// 内容語が残らないため未照合になる（そのまま表示させない）。
//
// spoken は SpokenContent で作ること（内容語だけの Bigrams）。
func ValidateEvidence(evidence map[string]string, spoken textsim.Bigrams) EvidenceCheck {
	var out EvidenceCheck
	for key, text := range evidence {
		if strings.TrimSpace(text) == "" {
			continue
		}
		out.Checked++
		if textsim.ContentBestMatch(text, spoken) < EvidenceMatchThreshold {
			out.Unmatched = append(out.Unmatched, key)
		}
	}
	// map の順序は不定なので、ログを安定させる
	sort.Strings(out.Unmatched)
	return out
}

// blankUnmatchedEvidence は照合できなかった項目の evidence を空文字にする（#1527）。
//
// キーごと削除せず空文字を入れるのは、キーの有無ではなく中身の有無で
// 「根拠を出せなかった」ことを表すため。スコアと講評はそのまま残す。
// 誤った根拠を載せるより欠落させる（docs/wiki/scoring.md §2-3）。
func blankUnmatchedEvidence(evidence map[string]string, unmatched []string) {
	for _, key := range unmatched {
		if _, ok := evidence[key]; ok {
			evidence[key] = ""
		}
	}
}
