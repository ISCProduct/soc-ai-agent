package interview

import (
	"regexp"
	"strings"
)

// STARElement は STAR フレームワークの各要素（#794）。
type STARElement string

const (
	// STARSituation は「いつ・どこで・どんな状況で」。
	STARSituation STARElement = "S"
	// STARTask は「何が課題だったか・何を求められたか」。
	STARTask STARElement = "T"
	// STARAction は「自分が何をしたか」。
	STARAction STARElement = "A"
	// STARResult は「どうなったか・どんな成果が出たか」。
	STARResult STARElement = "R"
)

// starLabels は欠落を人が読む形にするためのラベル。
var starLabels = map[STARElement]string{
	STARSituation: "状況",
	STARTask:      "課題",
	STARAction:    "行動",
	STARResult:    "成果",
}

// STARパターンは面接の日本語回答に合わせている。
//
// 日本語は主語を省くため「私は」に頼れない。動作・場面を表す語そのものを見る。
// 面接の回答は短く、1文に複数要素が入るので、文単位ではなく回答全体で判定する。
var starPatterns = map[STARElement]*regexp.Regexp{
	// 場面・所属・時期
	STARSituation: regexp.MustCompile(
		`大学|学校|専門|ゼミ|研究室|サークル|部活|アルバイト|バイト|インターン|前職|現職|授業|講義|` +
			`チーム|グループ|プロジェクト|案件|現場|会社|店舗|` +
			`[0-9０-９一二三四五六七八九]+年(生|次|間)?|入学|入社|当時|去年|昨年|昨夏`),
	// 課題・目標・制約
	STARTask: regexp.MustCompile(
		`課題|問題|目標|ミス|トラブル|不足|遅れ|間に合わ|難し|困難|バグ|障害|クレーム|` +
			`必要が|求められ|任され|依頼され|しなければ|しなくては|やらなければ|` +
			`目指し|挑戦|改善したい|効率化`),
	// 自分の行動
	STARAction: regexp.MustCompile(
		`実装|開発|作成|設計|構築|導入|提案|改善|修正|調整|交渉|説明|共有|相談|` +
			`担当|主導|推進|運用|検証|調査|分析|練習|勉強|学習|準備|声をかけ|巻き込`),
	// 成果・結果
	STARResult: regexp.MustCompile(
		`結果|成果|達成|完成|完了|実現|受賞|優勝|入賞|合格|採用され|評価され|表彰|` +
			`削減|短縮|向上|増加|改善(し|され|でき)|解決|好評|感謝され|リリース|公開|売上|` +
			`[0-9０-９]+\s*[%％]|[0-9０-９]+\s*(倍|件|人|万円|円|時間|分|日|週間|ヶ月|か月)`),
}

// STARAnalysis は回答に STAR の各要素が含まれるかと、その根拠。
type STARAnalysis struct {
	// Evidence は要素ごとの根拠となった語。含まれない要素はキーごと無い。
	Evidence map[STARElement]string
}

// Has は要素が含まれるか。
func (a STARAnalysis) Has(e STARElement) bool {
	_, ok := a.Evidence[e]
	return ok
}

// Missing は欠けている要素を、深掘りする優先順で返す。
//
// A（行動）を先に聞くのは、会話の順序がそうだから。
// 何をしたかが語られていない相手に「その取り組みの結果は？」と聞くと、
// 存在しない取り組みの成果を尋ねることになる。
// A が語られていれば、次に落としやすい R（成果）を聞く。
// S と T は文脈情報で、無くても回答の価値は残る。
func (a STARAnalysis) Missing() []STARElement {
	var missing []STARElement
	for _, e := range []STARElement{STARAction, STARResult, STARSituation, STARTask} {
		if !a.Has(e) {
			missing = append(missing, e)
		}
	}
	return missing
}

// AnalyzeSTAR は回答から STAR の各要素の有無と根拠を返す（#794）。
//
// キーワード一致による判定で、意味解析はしない。
// 「含まれない」と判定した要素を深掘りの対象にするだけなので、
// 取りこぼしても余分に質問するだけで、回答内容を壊さない。
func AnalyzeSTAR(answer string) STARAnalysis {
	trimmed := strings.TrimSpace(answer)
	result := STARAnalysis{Evidence: map[STARElement]string{}}
	if trimmed == "" {
		return result
	}
	for element, pattern := range starPatterns {
		if m := pattern.FindString(trimmed); m != "" {
			result.Evidence[element] = m
		}
	}
	return result
}

// starFollowUpQuestions は欠落要素ごとの深掘り質問。
//
// 「何を聞かれているか」が学生に伝わる文にする。
// 抽象的な「もう少し具体的に」では、何を足せばよいか分からず同じ回答が返る。
var starFollowUpQuestions = map[STARElement]string{
	STARResult:    "その取り組みの結果、どうなりましたか。数字や周囲の反応など、分かる範囲で具体的に教えてください。",
	STARAction:    "その中で、あなた自身が具体的にどう動いたのかを教えてください。",
	STARSituation: "それはいつ、どのような場面での話でしょうか。状況を教えてください。",
	STARTask:      "そのとき、何が課題だったのでしょうか。求められていたことを教えてください。",
}

// STARFollowUpQuestion は欠けている要素を狙う深掘り質問を返す。
//
// 次の2つの場合は空文字を返し、呼び出し側の従来テンプレートへ委ねる。
//
//   - STAR が揃っている（深掘りする要素が無い）
//   - **何ひとつ語られていない**（「はい。」「頑張りました。」など）
//
// 後者が重要で、STAR 分析は「エピソードは語られているが要素が欠けている」
// ときに効く道具である。何も語られていない相手に欠落を1つ選んで聞くと、
// 「はい。」に対して「その取り組みの結果、どうなりましたか」のような、
// 存在しない取り組みを前提にした質問になる。
// その場合は要素を絞らず、エピソードそのものを促す方がよい。
func STARFollowUpQuestion(answer string) (string, STARElement) {
	a := AnalyzeSTAR(answer)
	if len(a.Evidence) == 0 {
		return "", ""
	}
	missing := a.Missing()
	if len(missing) == 0 {
		return "", ""
	}
	return starFollowUpQuestions[missing[0]], missing[0]
}

// STARLabel は要素の日本語ラベル。ログや管理画面の表示用。
func STARLabel(e STARElement) string {
	return starLabels[e]
}
