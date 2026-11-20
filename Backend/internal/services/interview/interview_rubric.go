package interview

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
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
	// Levels は 0〜RubricScoreMax のレベル定義（アンカー）。添字がスコアそのもの。
	// プロンプトへそのまま入る（#1594）。
	Levels RubricLevels
}

// RubricLevels は1項目のレベル定義。添字 0〜RubricScoreMax がスコア。
//
// 配列だがコンパイラは書き忘れを止めない（短いコンポジットリテラルはゼロ値で
// 埋まる）。空のレベルを止めているのは TestRubricLevelsDefined。
//
// 書き方の制約は履歴書側（resume_rubric.go）と揃える。プロンプトが
// 「満たしている最も高いレベルを選ぶ」と指示しているので、どれが破れても
// 採点が意図と逆へ動く。
//
//  1. 隣り合うレベルの差が発言を見れば判定できること
//     （例: specificity の3点と4点の差は「具体が1種類か2種類以上か」）
//  2. 主観語だけで段を分けないこと（「優れている」「普通」では 3 と 4 の
//     区別をモデルに伝えられない）
//  3. 0〜2 は欠落の段・3 は基準を満たす段・4〜5 は積み増しという構えにすること
type RubricLevels [RubricScoreMax + 1]string

// RubricScoreMin / RubricScoreMax はスコアの値域。
// この範囲外は LLM の出力ミスとして弾く（#795）。
const (
	RubricScoreMin = 0
	RubricScoreMax = 5
)

// reportGenerationAttempts はレポート生成の試行回数（初回＋やり直し）。
// スキーマ違反は出力のばらつきによることが多く、1度やり直せば大抵通る。
// 増やしても費用が線形に増えるだけなので2に留める。
const reportGenerationAttempts = 2

// rubricCriteria は面接の評価項目。順序はプロンプトの記載順。
var rubricCriteria = []RubricCriterion{
	{"logic", "論理性", "回答が筋道立っているか、主張に一貫性があるか", RubricLevels{
		"質問に対応する答えが無い。話題が質問から外れたまま戻らない",
		"結論に当たる文は有るが、理由がまったく述べられていない",
		"結論と理由はあるが、理由が結論を支えていない。または前後で主張が食い違う",
		"結論と、それを支える理由が1つ述べられており、話の前後で主張が一貫している",
		"結論を支える理由が2つ以上あり、順序立てて述べられている。質問の条件（期間・立場など）にも答えている",
		"上記に加えて、反対の見方や制約に触れたうえで結論を選んだ理由が述べられている",
	}},
	{"specificity", "具体性", "具体的なエピソードや数値が含まれているか", RubricLevels{
		"具体が一切無く、「頑張りました」「成長できました」のような形容だけ",
		"活動名や所属だけがあり、自分が何をしたかの行動が述べられていない",
		"自分の行動は述べられているが、いつ・どこで・何を の具体がいずれも無い",
		"自分の行動に加えて、時期・場面・対象のいずれか1種類が述べられている",
		"自分の行動に加えて、時期・場面・対象・数量のうち2種類以上が述べられ、取組の規模と自分の役割が分かる",
		"上記に加えて、着手前→実施→結果の順で、聞き手が同じ状況を思い描ける粒度で述べられている",
	}},
	{"ownership", "主体性", "「私が〜した」という自分起点の表現があるか", RubricLevels{
		"主語が全て他者・組織・環境で、自分が何をしたかが現れない",
		"「参加した」「手伝った」のように、与えられた役割をこなしたことだけが述べられている",
		"自分の行動はあるが、すべて指示されたことの実行で、自分で決めた箇所が無い",
		"自分で判断して動いた箇所が1つ述べられている",
		"自分で課題を見つけ、やり方を決めて動いた経過が述べられている",
		"上記に加えて、うまくいかなかった案や断られた案を含めて、選び直した理由まで述べられている",
	}},
	{"communication", "コミュニケーション力", "簡潔・明確に伝えられているか、聞き返しが少ないか", RubricLevels{
		"何を言おうとしているか聞き取れない。文が途中で途切れたまま次へ移る",
		"話は続くが、主語や対象が省かれていて誰の何の話か分からない箇所が多い",
		"内容は分かるが、同じ説明の繰り返しや脱線が多く、要点に辿り着くまでが長い",
		"要点が先に述べられ、補足がその後に続く。聞き返さずに意味が取れる",
		"要点が先にあり、相手が知らない語には説明が添えられている。長さも質問の粒度に合っている",
		"上記に加えて、話の区切りで相手の理解を確かめる、または補足の要否を尋ねる姿勢が見える",
	}},
	{"enthusiasm", "積極性・熱意", "志望動機や意欲が伝わっているか。取り組みのきっかけや継続の姿勢も評価材料に含めてよい", RubricLevels{
		"志望や関心に触れた発言が無い",
		"「興味があります」「成長したいです」のように、気持ちの表明だけで対象が述べられていない",
		"関心の対象は述べられているが、なぜそう思ったかのきっかけが無い",
		"関心の対象と、そう思ったきっかけが述べられている",
		"きっかけに加えて、自分で調べた・試した・続けている事実が述べられている",
		"上記に加えて、その関心を応募先で何に使うかが、相手の事業や職種と結びつけて述べられている",
	}},
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
	lines = append(lines, "各項目について、**満たしている最も高いレベルの点数**を選ぶこと。")
	for _, c := range rubricCriteria {
		lines = append(lines, fmt.Sprintf("\n### %s（%s）\n%s", c.Key, c.Label, c.Description))
		for score, level := range c.Levels {
			lines = append(lines, fmt.Sprintf("- %d点: %s", score, level))
		}
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
// 発話に無い数値を含むものだけは hasUnspokenNumber で別に弾く（#1596）。
// 発話に無いカタカナ・英字の語（技術名・店の種類など）を含むものも hasUnspokenTerm で弾く（SOCAIAGENT-675）。
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

// EvidenceReferenceCheck は引用文と発話IDを照合した結果。
type EvidenceReferenceCheck struct {
	EvidenceCheck
	UtteranceIDs map[string]uint
}

// ValidateEvidenceReferences は引用が指定IDの受験者発話に基づくことを検証する（#1775）。
//
// 照合そのものは ValidateEvidence に任せ、照合先を指定された発話1件に絞る。
// 部分文字列の一致だと「はい」や「私」が根拠として通り、逆に全角半角や空白のゆれで
// 正当な引用が落ちる。実測済みのしきい値と未発話の数値の検出をそのまま使う。
func ValidateEvidenceReferences(
	evidence map[string]string,
	utteranceIDs map[string]*uint,
	utterances []models.InterviewUtterance,
) EvidenceReferenceCheck {
	byID := make(map[uint]models.InterviewUtterance, len(utterances))
	for _, utterance := range utterances {
		if utterance.Role == "user" {
			byID[utterance.ID] = utterance
		}
	}

	result := EvidenceReferenceCheck{UtteranceIDs: make(map[string]uint)}
	for _, key := range RubricKeys() {
		quote := evidence[key]
		id := utteranceIDs[key]
		if strings.TrimSpace(quote) == "" {
			if id != nil {
				result.Checked++
				result.Unmatched = append(result.Unmatched, key)
			}
			continue
		}
		result.Checked++
		if id == nil || *id == 0 {
			result.Unmatched = append(result.Unmatched, key)
			continue
		}
		utterance, ok := byID[*id]
		if !ok {
			result.Unmatched = append(result.Unmatched, key)
			continue
		}
		spoken := SpokenContent([]models.InterviewUtterance{utterance})
		if check := ValidateEvidence(map[string]string{key: quote}, spoken); len(check.Unmatched) > 0 {
			result.Unmatched = append(result.Unmatched, key)
			continue
		}
		result.UtteranceIDs[key] = *id
	}
	sort.Strings(result.Unmatched)
	return result
}

func evidenceIDsAsPointers(ids map[string]uint) map[string]*uint {
	out := make(map[string]*uint, len(ids))
	for key, id := range ids {
		value := id
		out[key] = &value
	}
	return out
}

func evidenceIDsFromPointers(ids map[string]*uint) map[string]uint {
	out := make(map[string]uint, len(ids))
	for key, id := range ids {
		if id != nil && *id > 0 {
			out[key] = *id
		}
	}
	return out
}

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
func SpokenContent(utterances []models.InterviewUtterance) SpokenEvidence {
	var b strings.Builder
	for _, u := range utterances {
		if u.Role != "user" {
			continue
		}
		b.WriteString(u.Text)
		b.WriteString("\n")
	}
	numbers := map[string]bool{}
	for _, n := range extractNumbers(b.String()) {
		numbers[n] = true
	}
	normalized := normalizeForTerms(b.String())
	return SpokenEvidence{Content: textsim.NewContent(b.String()), numbers: numbers, normalized: normalized, words: asciiWords(normalized)}
}

// SpokenEvidence は受験者発話の照合用の表現。
type SpokenEvidence struct {
	// Content は内容語だけの文字bigram（textsim.ContentBestMatch 用）。
	Content textsim.Bigrams
	// numbers は発話に出た数値（正規化済み。漢数字も算用数字にしたもの）。
	numbers map[string]bool
	// normalized は発話の全文を、語の照合用に幅と大文字小文字をそろえたもの（SOCAIAGENT-675）。
	normalized string
	// words は発話の英字の語の集合（英字の語は完全一致で照合する）。
	words map[string]bool
}

var numberPattern = regexp.MustCompile(`([0-9０-９]+(?:[,，.．][0-9０-９]+)*)(万|億|割)?`)

// extractNumbers は数値を正規化して返す（全角→半角、桁区切りのカンマを除去）。
//
// 漢数字も算用数字にして返す（SOCAIAGENT-675。三倍→"3"、五十人→"50"）。発話が漢数字で
// evidence が算用数字だと正当な引用が未照合になり、逆に evidence 側の漢数字は照合されず
// 捏造した数値が通っていた。数値にするのは助数詞つきと位取りの並びだけ（extractKanjiNumbers）。
func extractNumbers(s string) []string {
	out := extractKanjiNumbers(s)
	for _, sub := range numberPattern.FindAllStringSubmatch(s, -1) {
		var b strings.Builder
		for _, r := range sub[1] {
			switch {
			case r >= '０' && r <= '９':
				b.WriteRune('0' + (r - '０'))
			case r == ',' || r == '，':
			case r == '．':
				b.WriteRune('.')
			default:
				b.WriteRune(r)
			}
		}
		// 「200万円」と「二百万円」を同じ数値にする（漢数字側は万・億を掛けた値になるため）
		out = append(out, scaleArabicNumber(b.String(), sub[2]))
		// 割は百分率でも比べられるよう、10倍の値も入れる（3割 と 30%）
		if sub[2] == "割" {
			if v, err := strconv.ParseFloat(b.String(), 64); err == nil && v*10 == float64(int64(v*10)) {
				out = append(out, strconv.FormatInt(int64(v*10), 10))
			}
		}
	}
	return out
}

// scaleArabicNumber は算用数字に続く万・億を掛けた値を返す（"200"+"万"→"2000000"）。
// 掛けられない値（小数の端数が残るなど）はそのまま返す。
func scaleArabicNumber(n, unit string) string {
	var mul int64
	switch unit {
	case "万":
		mul = 10_000
	case "億":
		mul = 100_000_000
	default:
		return n
	}
	if v, err := strconv.ParseFloat(n, 64); err == nil {
		scaled := v * float64(mul)
		if scaled == float64(int64(scaled)) {
			return strconv.FormatInt(int64(scaled), 10)
		}
	}
	return n
}

// hasUnspokenNumber は evidence に、受験者が発話していない数値があるかを返す（#1596）。
//
// 内容語を発話から流用して数値だけ入れ替えた捏造（「部費を3倍にした」）は、
// 局所類似度では正当な要約と同じ帯に入り、しきい値で分離できない。
// 数値は完全一致で別に照合する。学生が言っていない数値は最も信じられやすい捏造になる。
func hasUnspokenNumber(text string, spoken SpokenEvidence) bool {
	for _, n := range extractNumbers(text) {
		if !spoken.numbers[n] {
			return true
		}
	}
	return false
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
func ValidateEvidence(evidence map[string]string, spoken SpokenEvidence) EvidenceCheck {
	return validateEvidence(evidence, spoken, false)
}

// ValidateTeacherEvidence は教員向けの根拠（teacher.detailed_evidence）を照合する（SOCAIAGENT-675）。
//
// 教員向けは「引用＋指導ポイント」なので、数値とカタカナ・英字の語は「」の引用の中だけで照合する
// （quotedOnly）。講評の「次は二点に分けて」「メンバー」は受験者の発言ではない。
// 内容語の類似度は今までどおり全文で見る。
func ValidateTeacherEvidence(evidence map[string]string, spoken SpokenEvidence) EvidenceCheck {
	return validateEvidence(evidence, spoken, true)
}

func validateEvidence(evidence map[string]string, spoken SpokenEvidence, teacher bool) EvidenceCheck {
	var out EvidenceCheck
	// キーは評価項目側から回す。evidence は LLM 出力の JSON をそのまま
	// unmarshal したものなので、キー自体もモデルが自由に書ける。
	// Unmatched は buildEvidenceRetryNote 経由でやり直しプロンプトの
	// 信頼領域へ連結されるため、キーに指示文を入れれば囲みを迂回できた（#1600）。
	// 評価項目に無いキーは読み出し側（OwnEvid 等）も使わないので捨てる。
	for _, key := range RubricKeys() {
		text, ok := evidence[key]
		if !ok || strings.TrimSpace(text) == "" {
			continue
		}
		out.Checked++
		claimed := text
		if teacher {
			claimed = quotedOnly(text)
		}
		if textsim.ContentBestMatch(text, spoken.Content) < EvidenceMatchThreshold ||
			hasUnspokenNumber(claimed, spoken) || hasUnspokenTerm(claimed, spoken) {
			out.Unmatched = append(out.Unmatched, key)
		}
	}
	// RubricKeys() は定義順で安定しているが、ログの比較を容易にするため並べ替えは残す
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
