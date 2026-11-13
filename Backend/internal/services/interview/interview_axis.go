package interview

import (
	"Backend/internal/models"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// 軸の選定しきい値（#2274 段階1）。値を変えるときは interview_axis_test.go も合わせて直す。
const (
	// maxTargetAxes は1回の面接で質問の軸にする人物像の軸数の上限。
	maxTargetAxes = 3
	// minAxisScore は軸として選ぶ重視度の下限。全軸が50付近のプロファイル
	// （デフォルト値のまま）から軸を選ばないための足切り。
	minAxisScore = 60
	// minAxisSpread は最大と最小の差がこれ未満のプロファイルを「識別できない」とみなす下限。
	// 全軸が似た値だと、上位3軸を選んでも根拠のない順位付けになる。
	minAxisSpread = 20
)

// interviewAxis は CompanyWeightProfile の1軸と、その軸を行動事象型で聞くための材料。
type interviewAxis struct {
	Key   string // interview_question_states.target_axis に保存する値
	Label string // 日本語の軸名（面接官のプロンプト用。学生には読み上げない）
	// Question は軸なしでも成り立つ定型の行動事象型質問。企業名・企業の事実を含めない。
	// キューに保存する質問文であり、差別質問検査に掛かったときのフォールバックでもある。
	Question string
	// Look はこの軸で見たい行動（出来事・役割・判断・結果のどこに注目するか）。
	Look  string
	score func(p *models.CompanyWeightProfile) int
}

// CategoryLabel は質問キューの Category に使う表示名。
func (a interviewAxis) CategoryLabel() string { return "人物像・" + a.Label }

// interviewAxes は軸の表。同点のときの順序（安定ソート）もこの並びで決まる。
var interviewAxes = []interviewAxis{
	{"technical", "技術志向", "これまでに技術的な課題に取り組んだ経験で、あなたが担った役割と、解決のために判断したことを教えてください。",
		"技術的な課題に向き合った出来事、自分で調べて選んだ手段、結果", func(p *models.CompanyWeightProfile) int { return p.TechnicalOrientation }},
	{"teamwork", "チームワーク", "チームで何かに取り組んだ経験で、あなたが担った役割と、チームがうまく進むよう工夫したことを教えてください。",
		"チーム内での自分の役割、周囲との協力の仕方、結果", func(p *models.CompanyWeightProfile) int { return p.TeamworkOrientation }},
	{"leadership", "リーダーシップ", "周りを巻き込んで物事を前に進めた経験で、あなたが取った行動とその結果を教えてください。",
		"自分から動いた場面、方針を決めた判断、周囲の巻き込み方、結果", func(p *models.CompanyWeightProfile) int { return p.LeadershipOrientation }},
	{"creativity", "創造性", "既存のやり方を見直したり、新しい工夫を取り入れたりした経験を、そのきっかけと結果とあわせて教えてください。",
		"工夫のきっかけ、自分が考えた案、試した結果", func(p *models.CompanyWeightProfile) int { return p.CreativityOrientation }},
	{"stability", "安定志向", "地道な取り組みを粘り強く続けた経験で、続けるために工夫したことと得られた結果を教えてください。",
		"継続した期間、続ける工夫、途中で崩れかけたときの対応、結果", func(p *models.CompanyWeightProfile) int { return p.StabilityOrientation }},
	{"growth", "成長志向", "新しい知識やスキルを身につけた経験で、きっかけと学び方、その後どう活かしたかを教えてください。",
		"学ぶきっかけ、自分で決めた学び方、その後の変化", func(p *models.CompanyWeightProfile) int { return p.GrowthOrientation }},
	{"work_life_balance", "時間管理・セルフマネジメント", "複数のことを並行して進めた経験で、優先順位をどう決めて両立させたかを教えてください。",
		"優先順位を決めた基準、時間や負荷の調整、結果", func(p *models.CompanyWeightProfile) int { return p.WorkLifeBalance }},
	{"challenge", "チャレンジ", "難しいと感じながらも挑戦した経験で、どう取り組み、どんな結果になったかを教えてください。",
		"挑戦を決めた理由、困難への向き合い方、失敗や結果からの学び", func(p *models.CompanyWeightProfile) int { return p.ChallengeSeeking }},
	{"detail", "丁寧さ・細部への注意", "ミスを防ぐために丁寧に確認した経験や、細かい点に気づいて改善した経験を教えてください。",
		"確認の工夫、気づいた場面、防げたことや改善した結果", func(p *models.CompanyWeightProfile) int { return p.DetailOrientation }},
	{"communication", "コミュニケーション", "相手の立場や考えが違う中で、意思疎通をうまく進めた経験と、そのとき工夫したことを教えてください。",
		"相手に合わせた伝え方、食い違いが生じた場面での対応、結果", func(p *models.CompanyWeightProfile) int { return p.CommunicationSkill }},
}

// axisByKey は軸キーから軸を返す。未知のキーはゼロ値（Key 空）。
func axisByKey(key string) interviewAxis {
	for _, a := range interviewAxes {
		if a.Key == key {
			return a
		}
	}
	return interviewAxis{}
}

// SelectTargetAxes は企業×職種の人物像から、質問の軸にする上位最大 maxTargetAxes 軸を選ぶ。
// プロファイルが無い・全軸が似た値（最大-最小が minAxisSpread 未満）・全軸が minAxisScore 未満
// のときは nil を返し、呼び出し側は従来の標準トピックのままにする。
// 決定的なコードで選び、LLM は呼ばない。
func SelectTargetAxes(profile *models.CompanyWeightProfile) []interviewAxis {
	if profile == nil {
		return nil
	}
	lo, hi := 100, 0
	for _, a := range interviewAxes {
		s := a.score(profile)
		lo, hi = min(lo, s), max(hi, s)
	}
	if hi-lo < minAxisSpread {
		return nil
	}
	ranked := slices.Clone(interviewAxes)
	slices.SortStableFunc(ranked, func(x, y interviewAxis) int { return y.score(profile) - x.score(profile) })
	var out []interviewAxis
	for _, a := range ranked {
		if len(out) == maxTargetAxes || a.score(profile) < minAxisScore {
			break
		}
		out = append(out, a)
	}
	return out
}

// buildAxisDirectiveBlock は軸スロットの質問用に面接官へ渡す指示。
// 人物像は LLM による相対評価の推定値で、企業が公式に述べた要件ではない（#2123）。
// そのため企業の求める人物像を断定させず、事実として引用してよいのは【企業情報】にある項目だけにする。
// 軸の名前は学生へ読み上げさせない（評価観点の開示になる）。
func buildAxisDirectiveBlock(axis interviewAxis, baseQuestion string) string {
	return fmt.Sprintf(`【今回の質問（行動事象型）】
基本の質問: %s
- 上記を土台に、「%s」を示す具体的な出来事を1つ聞いてください（その出来事・自分の役割・自分で下した判断・結果）。見たい点: %s
- 応募者の履歴書が【応募者の履歴書】として渡されていて、そこに具体的な出来事があれば、それを起点に聞いてください。なければ上の基本の質問を自然な言い回しで聞いてください
- 経歴が薄い応募者も不利にしないでください。学業・ゼミ・サークル・アルバイト・個人の活動も材料にしてよいと伝えてください
- この質問で見たい資質の名前（「%s」など）は応募者に伝えない。評価・講評もしない
- 「御社は〜を求めている」「弊社は〜な人を求めている」のように、企業の求める人物像を断定しない
- 企業の事実として触れてよいのは【企業情報】に書かれている項目（事業・文化・働き方・技術スタック・求人の必須スキル等）だけです。その軸に結びつく事実が【企業情報】に無ければ、企業名や企業の事情に触れず、一般の行動質問として聞いてください`,
		baseQuestion, axis.Label, axis.Look, axis.Label)
}

// ---- 就職差別につながる質問の検査（#2274） ----

// 学生の回答ではなくアシスタント（面接官）側の生成文にだけ使う。
// ponytail: 正規表現による決定的な検査。言い換え・婉曲表現は拾えない。取りこぼしが問題になれば語を足す。
var discriminatoryPatterns = []*regexp.Regexp{
	// 家族構成・家庭環境
	regexp.MustCompile(`家族(の)?構成|家族(は|の)?(何人|人数|職業|仕事|年収|状況)|(ご?両親|お父さん|お母さん|父親|母親|親)の(職業|仕事|勤務先|年収)|ご?兄弟(姉妹)?(は|の|構成)`),
	// 出身地・本籍・国籍
	regexp.MustCompile(`出身地|ご?出身(は|の方|を)|どこの出身|どちらの出身|生まれ(は|た場所|育った)|本籍|国籍|何人(です|ですか)`),
	// 宗教・支持政党・思想
	regexp.MustCompile(`宗教|信仰(を|は|して)|支持(する)?政党|(どの|何)党|(政治|宗教|ご自身|あなた|個人)的?な?の?(思想|信条)|思想・信条`),
}

// 検査前に取り除く、差別質問ではない定型語（「出身大学」「宗教学」「設計思想」等）。
var benignCompounds = strings.NewReplacer(
	"出身校", "", "出身大学", "", "出身学部", "", "出身高校", "", "出身学科", "", "出身ゼミ", "", "出身研究室", "",
	"ご出身の大学", "", "ご出身の学部", "", "ご出身の高校", "", "ご出身の学校", "",
	"宗教学", "", "宗教史", "", "宗教法人", "", "設計思想", "", "開発思想", "",
	"家族のよう", "",
)

// ContainsDiscriminatoryQuestion は生成された質問文が、家族構成・出身地・本籍・国籍・宗教・
// 支持政党・思想など、適性・能力と関係のない事項を聞く表現を含むかを返す。
func ContainsDiscriminatoryQuestion(text string) bool {
	text = benignCompounds.Replace(text)
	for _, re := range discriminatoryPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// guardDiscriminatory は面接官の生成文に差別質問が含まれていたら、1回だけ再生成し、
// それでも含まれる（または再生成に失敗した）ときは軸なしの定型質問に差し替える。
// 学生の回答には使わない。
func guardDiscriminatory(text string, regenerate func() (string, error)) string {
	if !ContainsDiscriminatoryQuestion(text) {
		return text
	}
	if again, err := regenerate(); err == nil && !ContainsDiscriminatoryQuestion(again) {
		return again
	}
	return genericFallbackQuestion
}

// axisKeyOf は質問指示の軸キー。directive が nil なら空。
func axisKeyOf(d *questionDirective) string {
	if d == nil {
		return ""
	}
	return d.TargetAxis
}

// genericFallbackQuestion は差別質問検査を2回通らなかったときに使う、軸なしの定型質問。
const genericFallbackQuestion = "これまでの経験の中で、特に力を入れて取り組んだことと、その中でのあなたの役割を教えてください。"
