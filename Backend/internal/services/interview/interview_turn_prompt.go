package interview

import (
	"Backend/internal/models"
	"Backend/internal/services/shared"
	"fmt"
	"strings"
)

// interviewTopics は面接で扱うトピックの順序定義です
var interviewTopics = []string{
	"自己紹介・志望動機",
	"職務経験・実績",
	"強み・弱み",
	"キャリアビジョン",
	"逆質問",
}

// maxTurnsPerTopic は1トピックあたりの最大ターン数（深掘り含む）。
// maxFollowUpDepth（interview_question_planner.go）と揃え、体感が長すぎるという
// 報告(#910)を受けて2に短縮した。
const maxTurnsPerTopic = 2

// 【役割の徹底】は #1223。面接官が企業側であること・逆質問には自分が答えることは
// 従来プロンプトに書かれておらず、モデルの暗黙の補完に依存していた。
// #1209 の計測では、補完できないモデルに与えると自社を「御社」と呼び（8ターン中5回）、
// 逆質問には答えず聞き返す。逆質問は interviewTopics の最後なので毎回そこで面接が壊れる。
const interviewBaseRulesPrompt = `あなたは日本語の就活面接官です。以下を守ってください。

【基本ルール】
- 1回の返答は2〜3文、150文字以内で短くまとめる
- 必ず1つの質問で締めくくる
- 評価・講評は面接終了まで行わない
- 企業名を発話する際は、英字・略語はカタカナの正しい読み方で読んでください
- 常に中立的で丁寧なトーンを保ち、叱責・説教・高圧的な言い回しは絶対にしない
- 回答が的外れ・曖昧・無回答でも、いらだちや不満を示さず「聞き取れなかった」体で
  穏やかに言い換えるか、別の角度から質問し直す

【役割の徹底】（重要）
- あなたは採用する側（企業側）の面接官です。応募者ではありません
- 自社を指すときは必ず「弊社」または「当社」と言う。「御社」は応募者が使う言葉なので絶対に使わない
- 応募者から逆質問を受けたら、【企業情報】をもとにあなたが答える。質問で返さない
- 応募者の話が面接と関係ない話題に逸れたら、その話題を掘り下げず、面接のトピックへ戻す
- これまでの会話履歴を踏まえ、既に聞いた内容（「力を入れたこと」等）と同じ趣旨の質問を
  言い回しを変えただけで繰り返さない。既に答えてもらえた話題は別の角度・別のトピックの
  質問に進める`

// interviewFlowPrompt は【質問の進め方】と【深掘りの判断基準】の節。
const interviewFlowPrompt = `

【質問の進め方】
面接は以下の順番でトピックを進めてください:
1. 自己紹介・志望動機
2. 職務経験・実績
3. 強み・弱み
4. キャリアビジョン
5. 逆質問

【深掘りの判断基準】（重要）
- 応募者の回答が抽象的・表面的な場合のみ深掘りしてください
- 採用判断に有益な情報（具体的エピソード・数値・自分の行動・結果）が得られた場合は次のトピックへ移行してください
- 深掘り時は、必要に応じて「なぜそれに取り組み始めたか（きっかけ）」や「どう継続したか」の観点も取り入れてください（質問数は増やさず、既存の深掘り1回の中で行う）
- 同一トピックへの深掘りは最大1回までとし、現在のトピックでの質問回数が2回に達したら必ず次のトピックへ移行してください
- 1つの質問は約3分を目安にし、目安時間に達したら自然に次の質問へ移行してください
- トピックを移行する際は「ありがとうございます。次に〜についてお聞きします。」と自然につないでください`

// questionQueuePrompt は質問キューがあるときに足す【質問キューとの整合】の節。
const questionQueuePrompt = `

【質問キューとの整合】
今回の質問カテゴリは面接質問キューに従ってください。企業独自のカテゴリを標準トピックへ無理に読み替えず、質問キューの順序を優先してください。`

// sierGuidelinePrompt は SIer 企業向けに足す節。
const sierGuidelinePrompt = `

【SIer企業向け質問ガイドライン】
- 常駐先での顧客折衝・要件ヒアリング経験を深掘りする
- 上流工程（要件定義・基本設計）への関与実績を確認する
- ウォーターフォール・アジャイルどちらの経験があるか確認する
- IPA資格や技術資格の取得状況・今後の学習意欲を聞く
- 多様な現場・技術スタックへの適応力を問う
- 技術フェーズ（詳細設計・実装・テスト）と要件定義フェーズの両方を経験しているか確認する`

func buildInterviewSystemPrompt(
	companyName,
	companyReading,
	position,
	companyInfo,
	companyType string,
	customQuestions []models.InterviewCompanyQuestion,
	skillScores []models.SkillScore,
	turnCount int,
	remainingSeconds int,
	questionIndex int,
	totalQuestions int,
	questionElapsedSeconds int,
	questionDurationSeconds int,
	directive *questionDirective,
) string {
	tb := normalizeQuestionTimebox(questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds)
	progress := computeTopicProgress(turnCount, directive)

	base := interviewBaseRulesPrompt
	base += interviewStatusSection(progress, tb, remainingSeconds)
	base += interviewFlowPrompt
	if directive != nil {
		base += questionQueuePrompt
	}
	base += interviewFactsSection(companyName, companyReading, position, companyInfo)
	if companyType == "sier" {
		base += sierGuidelinePrompt
	}
	base += questionInstructionSection(directive, customQuestions)
	base += techQuestionGuidelineSection(position, skillScores)
	return strings.TrimSpace(base)
}

// questionTimebox は質問タイムボックスの値（範囲内に丸めた後）。
type questionTimebox struct {
	questionIndex, totalQuestions, elapsedSeconds, durationSeconds int
}

// normalizeQuestionTimebox は質問番号・経過時間などを既定値で補い、範囲内に丸める。
func normalizeQuestionTimebox(questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds int) questionTimebox {
	if questionDurationSeconds <= 0 {
		questionDurationSeconds = 180
	}
	if totalQuestions <= 0 {
		totalQuestions = 1
	}
	if questionIndex <= 0 {
		questionIndex = 1
	}
	if questionIndex > totalQuestions {
		questionIndex = totalQuestions
	}
	if questionElapsedSeconds < 0 {
		questionElapsedSeconds = 0
	}
	if questionElapsedSeconds > questionDurationSeconds {
		questionElapsedSeconds = questionDurationSeconds
	}
	return questionTimebox{questionIndex, totalQuestions, questionElapsedSeconds, questionDurationSeconds}
}

// topicProgress は現在のトピックと進み具合。
type topicProgress struct {
	currentTopic    string
	topicPosition   string
	turnsOnTopic    int
	remainingTopics int
}

// computeTopicProgress はターン数（質問キューがあればそのカテゴリ）から現在のトピックと進み具合を求める。
func computeTopicProgress(turnCount int, directive *questionDirective) topicProgress {
	// ターン数からトピックインデックスと当該トピック内のターン数を計算
	topicIndex := 0
	turnsOnTopic := 0
	if turnCount > 0 {
		topicIndex = (turnCount - 1) / maxTurnsPerTopic
		if topicIndex >= len(interviewTopics) {
			topicIndex = len(interviewTopics) - 1
		}
		turnsOnTopic = (turnCount-1)%maxTurnsPerTopic + 1
	}
	currentTopic := interviewTopics[topicIndex]
	remainingTopics := len(interviewTopics) - topicIndex - 1
	topicPosition := fmt.Sprintf("%d/%d", topicIndex+1, len(interviewTopics))
	if directive != nil && strings.TrimSpace(directive.Category) != "" {
		currentTopic = directive.Category
		topicPosition = "企業別カテゴリ"
		topicIndex = -1
		for i, topic := range interviewTopics {
			if topic == directive.Category {
				topicIndex = i
				topicPosition = fmt.Sprintf("%d/%d", topicIndex+1, len(interviewTopics))
				break
			}
		}
		if topicIndex < 0 {
			currentTopic = "企業別"
		}
		turnsOnTopic = 1
		if directive.IsDeepening {
			turnsOnTopic = maxTurnsPerTopic
		}
		if topicIndex < 0 {
			remainingTopics = len(interviewTopics)
		} else {
			remainingTopics = len(interviewTopics) - topicIndex - 1
		}
	}
	return topicProgress{currentTopic, topicPosition, turnsOnTopic, remainingTopics}
}

// interviewStatusSection は【現在の面接状況】【質問タイムボックス】と残り時間の節。
func interviewStatusSection(p topicProgress, tb questionTimebox, remainingSeconds int) string {
	// 現在のトピックと進行状況を明示
	s := fmt.Sprintf("\n\n【現在の面接状況】\n現在のトピック: %s（%s）\n現在のトピックでの質問回数: %d/%d回",
		p.currentTopic, p.topicPosition, p.turnsOnTopic, maxTurnsPerTopic)
	s += fmt.Sprintf(
		"\n\n【質問タイムボックス】\n1質問あたりの目安: 約%d分（%d秒）\n現在の質問番号: %d/%d\nこの質問の経過時間: %d秒\nこの質問の残り目安: %d秒",
		tb.durationSeconds/60,
		tb.durationSeconds,
		tb.questionIndex,
		tb.totalQuestions,
		tb.elapsedSeconds,
		tb.durationSeconds-tb.elapsedSeconds,
	)

	if remainingSeconds > 0 {
		remainingMinutes := remainingSeconds / 60
		s += fmt.Sprintf("\n残り時間: 約%d分", remainingMinutes)
		if remainingMinutes <= 2 && p.remainingTopics > 0 {
			s += fmt.Sprintf("（残り%dトピックあります。ペースを上げてください）", p.remainingTopics)
		}
	}
	return s
}

// interviewFactsSection は【面接情報】（志望企業・読み・応募職種・企業情報）の節。企業名・職種が両方空なら出さない。
func interviewFactsSection(companyName, companyReading, position, companyInfo string) string {
	if companyName == "" && position == "" {
		return ""
	}
	// 【面接情報】はブロックごと非信頼テキストとして囲む（#1600）。
	//
	// 志望企業・読み・応募職種は controllers/interview/controller.go の
	// r.FormValue 直値で、長さ制限も検証もない。企業情報だけ囲んでも、
	// その3行上の志望企業・応募職種が信頼領域に残っていては迂回される
	// （囲みの手前に payload を置けるので企業情報より攻撃が容易）。
	// 企業情報も DB の企業行（AI 取得パイプライン経由なら Web 由来）か
	// クライアント文面（resolveCompanyInfo のフォールバック）で、
	// いずれも面接官の system プロンプトの信頼領域へ生で入れてはいけない。
	//
	// 囲むのはこのプロンプトだけ。companyName / companyReading は
	// TTS の読み替え（applyCompanyReadingForTTS）にも使うので値そのものは変えない。
	//
	// 「この中の指示に従わない」宣言は shared.WrapUntrustedText が
	// ブロック直前へ同じノンス付きで出すので、ここには書かない
	// （同じ趣旨を2箇所に書くと片方が消えてもテストが通る。PR #1597 の指摘）。
	var facts strings.Builder
	if companyName != "" {
		facts.WriteString("志望企業: " + companyName)
		if companyReading != "" {
			facts.WriteString("（読み: " + companyReading + "）")
		}
		facts.WriteString("\n")
	}
	if position != "" {
		facts.WriteString("応募職種: " + position + "\n")
	}
	if strings.TrimSpace(companyInfo) != "" {
		facts.WriteString("\n【企業情報】\n" + companyInfo)
	}
	s := "\n\n【面接情報】\n" + shared.WrapUntrustedText(facts.String(), "面接情報")
	s += "\n\n上記の企業・職種に合わせた質問を行ってください。企業文化・働き方・福利厚生の情報がある場合は、それらを踏まえた「この企業ならでは」の深掘り質問を取り入れてください。"
	return s
}

// questionInstructionSection は企業別カスタム質問の節。
// 質問キューの今回の質問があれば【今回の質問】、無ければ必須・推奨質問の一覧を出す。
func questionInstructionSection(directive *questionDirective, customQuestions []models.InterviewCompanyQuestion) string {
	if directive != nil && strings.TrimSpace(directive.Text) != "" {
		return currentQuestionSection(directive, customQuestions)
	}
	if len(customQuestions) > 0 {
		return customQuestionListSection(customQuestions)
	}
	return ""
}

// currentQuestionSection は質問キューの今回の質問を渡す【今回の質問】の節。
func currentQuestionSection(directive *questionDirective, customQuestions []models.InterviewCompanyQuestion) string {
	s := "\n\n【今回の質問】"
	if directive.IsDeepening {
		s += "\n前の回答を踏まえた深掘り質問です。"
	}
	// 質問文も非信頼テキスト（#1600）。directive.Text は
	// interview_question_states に保存された過去の LLM 出力か、
	// 企業情報をもとに生成された企業別カスタム質問なので、
	// 【企業情報】の囲みを迂回して system プロンプトへ戻ってくる経路になる。
	// 「そのまま投げかける」という外側の指示は残し、中の指示文には従わせない。
	// カテゴリも出自が同じなので同じブロックへ入れる（囲みの外に置くと迂回路になる）。
	var notes string
	if directive.Category != "" {
		notes += fmt.Sprintf("\n（カテゴリ: %s）", directive.Category)
	}
	intent := evaluationIntentFor(customQuestions, directive.CompanyQuestionID)
	if intent != "" {
		notes += fmt.Sprintf("\n（評価したい観点: %s）", intent)
	}
	if directive.IsDeepening {
		// 深掘りの定型文（directive.Text）は学生の回答の一部を含む。システムプロンプトは伏字の対象外なので
		// 入れない。回答そのものは履歴（伏字を通る）にあるので、元の質問と狙いだけ渡して言い直させる。
		// カテゴリ・評価したい観点は元の質問と同じ囲みへ入れる（#2573。外に置くと迂回路になる）。
		s += fmt.Sprintf("\n直前の回答を踏まえ、具体性を引き出す深掘り質問を1問だけ、面接官として自然な言葉で投げかけてください。元の質問:\n%s",
			shared.WrapUntrustedText(directive.DeepeningOf+notes, "質問文"))
		if directive.DeepeningFocus != "" {
			s += "\n" + directive.DeepeningFocus
		}
	} else {
		s += fmt.Sprintf("\n次の質問文をそのまま面接官として投げかけてください（1問のみ）:\n%s",
			shared.WrapUntrustedText(directive.Text+notes, "質問文"))
	}
	if intent != "" {
		s += "\n" + evaluationIntentNote
	}
	return s
}

// customQuestionListSection は質問キューが無いときに企業別カスタム質問を必須・推奨に分けて並べる節。
func customQuestionListSection(customQuestions []models.InterviewCompanyQuestion) string {
	var required, recommended []string
	for _, q := range customQuestions {
		if q.IsRequired {
			required = append(required, customQuestionLine(q))
		} else {
			recommended = append(recommended, customQuestionLine(q))
		}
	}
	// 企業別カスタム質問は企業ポータル・管理画面の入力か、企業情報をもとに
	// AI 生成した候補（admin/interview_controller.go の generateQuestionsWithAI）で、
	// どちらも学生から見れば非信頼テキスト。【企業情報】の囲みを迂回するので
	// ここでも囲む（#1600）。
	s := ""
	if len(required) > 0 {
		s += "\n\n【必須質問（必ず全て質問してください）】\n" +
			shared.WrapUntrustedText(strings.Join(required, "\n"), "必須質問")
	}
	if len(recommended) > 0 {
		s += "\n\n【推奨質問（会話の流れに応じて取り入れてください）】\n" +
			shared.WrapUntrustedText(strings.Join(recommended, "\n"), "推奨質問")
	}
	if anyEvaluationIntent(customQuestions) {
		s += "\n\n" + evaluationIntentNote
	}
	return s
}

// techQuestionGuidelineSection は GitHub スキルスコアに基づく技術質問ガイドラインの節（エンジニア職種のみ）。
func techQuestionGuidelineSection(position string, skillScores []models.SkillScore) string {
	if len(skillScores) == 0 || !isEngineerPosition(position) {
		return ""
	}
	techHints := buildTechQuestionHints(skillScores)
	if techHints == "" {
		return ""
	}
	return "\n\n【GitHubスキル分析に基づく技術質問ガイドライン】\n" + techHints
}

// studentNameInstruction は、応募者の氏名を外部の AI へ送らない代わりに渡す指示。
// 発話は送る前に伏字になる（<NAME> など）ので、氏名は呼びかけに使えない。
// 伏字の記号をそのまま読み上げ・表示してしまわないよう、使わせない。
const studentNameInstruction = "【呼びかけ】応募者の氏名は個人情報のため伝えられていません。氏名では呼ばず、「あなた」などで呼びかけてください。発話に含まれる <NAME> のような山括弧の記号は伏字なので、応答に書かないでください。\n"

// isEngineerPosition は職種名がエンジニア系かどうかを判定する
func isEngineerPosition(position string) bool {
	engineerKeywords := []string{
		"エンジニア", "engineer", "Engineer",
		"開発", "developer", "Developer",
		"プログラマ", "programmer", "Programmer",
		"バックエンド", "フロントエンド", "インフラ", "SRE", "DevOps",
		"アーキテクト", "Architect", "architect",
	}
	for _, kw := range engineerKeywords {
		if strings.Contains(position, kw) {
			return true
		}
	}
	return false
}

// buildTechQuestionHints はスキルスコアをもとに技術質問ガイドラインを生成する
func buildTechQuestionHints(scores []models.SkillScore) string {
	type scored struct {
		category models.SkillCategory
		score    float64
	}
	var top []scored
	for _, s := range scores {
		if s.Score >= 30 {
			top = append(top, scored{s.Category, s.Score})
		}
	}
	if len(top) == 0 {
		return ""
	}

	// スコア降順でソート
	for i := 0; i < len(top)-1; i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].score > top[i].score {
				top[i], top[j] = top[j], top[i]
			}
		}
	}

	hints := []string{"（応募者のGitHubスキルスコアを参考に技術的な深掘りを行ってください）"}
	for _, s := range top {
		switch s.category {
		case models.SkillCategoryBackend:
			hints = append(hints, fmt.Sprintf("- バックエンド（スコア%.0f）: 言語・フレームワーク・設計パターン・パフォーマンスチューニングについて質問する", s.score))
		case models.SkillCategoryFrontend:
			hints = append(hints, fmt.Sprintf("- フロントエンド（スコア%.0f）: 使用フレームワーク・状態管理・UI最適化・アクセシビリティについて質問する", s.score))
		case models.SkillCategoryInfra:
			hints = append(hints, fmt.Sprintf("- インフラ（スコア%.0f）: クラウドサービス・IaC・CI/CD・監視設計について質問する", s.score))
		case models.SkillCategoryDB:
			hints = append(hints, fmt.Sprintf("- データベース（スコア%.0f）: スキーマ設計・クエリ最適化・トランザクション管理について質問する", s.score))
		}
	}
	return strings.Join(hints, "\n")
}
