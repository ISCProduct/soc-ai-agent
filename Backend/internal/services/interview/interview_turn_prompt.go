package interview

import (
	"Backend/internal/models"
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

// maxTurnsPerTopic は1トピックあたりの最大ターン数（深掘り含む）
const maxTurnsPerTopic = 3

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

	base := `あなたは日本語の就活面接官です。以下を守ってください。

【基本ルール】
- 1回の返答は2〜3文以内で短くまとめる
- 必ず1つの質問で締めくくる
- 評価・講評は面接終了まで行わない
- 企業名を発話する際は、英字・略語はカタカナの正しい読み方で読んでください`

	// 現在のトピックと進行状況を明示
	base += fmt.Sprintf("\n\n【現在の面接状況】\n現在のトピック: %s（%d/%d）\n現在のトピックでの質問回数: %d/%d回",
		currentTopic, topicIndex+1, len(interviewTopics), turnsOnTopic, maxTurnsPerTopic)
	base += fmt.Sprintf(
		"\n\n【質問タイムボックス】\n1質問あたりの目安: 約%d分（%d秒）\n現在の質問番号: %d/%d\nこの質問の経過時間: %d秒\nこの質問の残り目安: %d秒",
		questionDurationSeconds/60,
		questionDurationSeconds,
		questionIndex,
		totalQuestions,
		questionElapsedSeconds,
		questionDurationSeconds-questionElapsedSeconds,
	)

	if remainingSeconds > 0 {
		remainingMinutes := remainingSeconds / 60
		base += fmt.Sprintf("\n残り時間: 約%d分", remainingMinutes)
		if remainingMinutes <= 2 && remainingTopics > 0 {
			base += fmt.Sprintf("（残り%dトピックあります。ペースを上げてください）", remainingTopics)
		}
	}

	base += `

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
- 同一トピックへの深掘りは最大2回までとし、現在のトピックでの質問回数が3回に達したら必ず次のトピックへ移行してください
- 1つの質問は約3分を目安にし、目安時間に達したら自然に次の質問へ移行してください
- トピックを移行する際は「ありがとうございます。次に〜についてお聞きします。」と自然につないでください`

	if companyName != "" || position != "" {
		base += "\n\n【面接情報】"
		if companyName != "" {
			companyLabel := companyName
			if companyReading != "" {
				companyLabel += "（読み: " + companyReading + "）"
			}
			base += "\n志望企業: " + companyLabel
		}
		if position != "" {
			base += "\n応募職種: " + position
		}
		if companyInfo != "" {
			base += "\n\n【企業情報】\n" + companyInfo
		}
		base += "\n\n上記の企業・職種に合わせた質問を行ってください。企業文化・働き方・福利厚生の情報がある場合は、それらを踏まえた「この企業ならでは」の深掘り質問を取り入れてください。"
	}

	if companyType == "sier" {
		base += `

【SIer企業向け質問ガイドライン】
- 常駐先での顧客折衝・要件ヒアリング経験を深掘りする
- 上流工程（要件定義・基本設計）への関与実績を確認する
- ウォーターフォール・アジャイルどちらの経験があるか確認する
- IPA資格や技術資格の取得状況・今後の学習意欲を聞く
- 多様な現場・技術スタックへの適応力を問う
- 技術フェーズ（詳細設計・実装・テスト）と要件定義フェーズの両方を経験しているか確認する`
	}

	// 企業別カスタム質問をシステムプロンプトに注入
	if directive != nil && strings.TrimSpace(directive.Text) != "" {
		base += "\n\n【今回の質問】"
		if directive.IsDeepening {
			base += "\n前の回答を踏まえた深掘り質問です。"
		}
		base += fmt.Sprintf("\n次の質問文をそのまま面接官として投げかけてください（1問のみ）:\n%s", directive.Text)
		if directive.Category != "" {
			base += fmt.Sprintf("\n（カテゴリ: %s）", directive.Category)
		}
	} else if len(customQuestions) > 0 {
		var required, recommended []string
		for _, q := range customQuestions {
			if q.IsRequired {
				required = append(required, fmt.Sprintf("- [%s] %s", q.Category, q.QuestionText))
			} else {
				recommended = append(recommended, fmt.Sprintf("- [%s] %s", q.Category, q.QuestionText))
			}
		}
		if len(required) > 0 {
			base += "\n\n【必須質問（必ず全て質問してください）】\n" + strings.Join(required, "\n")
		}
		if len(recommended) > 0 {
			base += "\n\n【推奨質問（会話の流れに応じて取り入れてください）】\n" + strings.Join(recommended, "\n")
		}
	}

	// GitHubスキルスコアに基づく技術質問ガイドラインを注入（エンジニア職種のみ）
	if len(skillScores) > 0 && isEngineerPosition(position) {
		techHints := buildTechQuestionHints(skillScores)
		if techHints != "" {
			base += "\n\n【GitHubスキル分析に基づく技術質問ガイドライン】\n" + techHints
		}
	}

	return strings.TrimSpace(base)
}

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
