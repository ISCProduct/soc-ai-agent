package models

import (
	"encoding/json"
	"gorm.io/gorm"
)

// SeedPredefinedQuestions 事前定義質問のシードデータ
func SeedPredefinedQuestions(db *gorm.DB) error {
	// 既存データを削除（強制的に再投入するため）
	db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&PredefinedQuestion{})
	println("Existing predefined questions deleted")

	// エンジニア職種のIDを取得（もし存在すれば）
	var engineerCategory JobCategory
	db.Where("code = ?", "ENG").First(&engineerCategory)
	var softwareEngCategory JobCategory
	db.Where("code = ?", "ENG-SW").First(&softwareEngCategory)
	var webEngCategory JobCategory
	db.Where("code = ?", "ENG-WEB").First(&webEngCategory)

	defaultPhases := "[\"job_analysis\",\"interest_analysis\",\"aptitude_analysis\",\"future_analysis\"]"
	questions := []PredefinedQuestion{
		// 技術志向の質問（エンジニア職種向け）
		{
			Category:     "技術志向",
			QuestionText: "プログラミングや技術的なことを学ぶのは好きですか？もし好きであれば、どんなことを学んできましたか？（授業、趣味、独学など何でも構いません）",
			TargetLevel:  "新卒",
			JobCategoryID: func() *uint {
				if engineerCategory.ID != 0 {
					return &engineerCategory.ID
				}
				return nil
			}(),
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"プログラミング", "コーディング", "開発", "アプリ", "Web", "システム",
				"Python", "Java", "JavaScript", "Go", "C", "Ruby",
				"ハッカソン", "技術書", "Qiita", "GitHub", "個人開発", "趣味", "独学",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "特にない", "苦手", "嫌い", "難しい",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"プログラミング", "開発", "アプリ", "コーディング"},
					ScoreChange: 3,
					Description: "技術的な学習経験あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"個人開発", "趣味", "独学", "自主的"},
					ScoreChange: 2,
					Description: "自主的な学習姿勢",
				},
				{
					Condition:   "length_gt",
					Keywords:    []string{"100"},
					ScoreChange: 1,
					Description: "具体的な説明ができている",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 1,
					Description: "具体例を含んでいる",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーがプログラミング経験について答えられなかったようです。もっと答えやすい形で、授業や趣味での経験を聞いてください。",
					Purpose:  "技術経験の深掘り",
				},
				{
					Trigger:  "high_score",
					UseAI:    true,
					AIPrompt: "ユーザーは技術に強い関心があるようです。その中で特に楽しかったプロジェクトや経験について聞いてください。",
					Purpose:  "技術志向の確認",
				},
			}),
			IsActive: true,
		},

		// 技術志向の質問（営業職種向け）
		{
			Category:     "技術志向",
			QuestionText: "最新のIT技術やツールを使って業務を効率化することに興味はありますか？（例えばChatGPTやExcel、便利なアプリなど）",
			TargetLevel:  "新卒",
			JobCategoryID: func() *uint {
				if engineerCategory.ID != 0 {
					var salesCategory JobCategory
					db.Where("code = ?", "SALES").First(&salesCategory)
					if salesCategory.ID != 0 {
						return &salesCategory.ID
					}
				}
				return nil
			}(),
			Priority:      9,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"効率化", "IT", "ツール", "ChatGPT", "AI", "Excel", "アプリ", "便利", "自動化",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"苦手", "使わない", "わからない", "面倒",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"効率化", "興味", "使ってみたい", "便利"},
					ScoreChange: 3,
					Description: "IT利活用への関心",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{}),
			IsActive:      true,
		},
		{
			Category:      "チームワーク志向",
			QuestionText:  "グループワークやチーム活動をした経験はありますか？その時、あなたはどんな役割でしたか？（授業、サークル、アルバイトなど何でも構いません）",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"協力", "サポート", "コミュニケーション", "相談", "分担", "チーム",
				"グループ", "メンバー", "助け合い", "連携", "協調",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "特にない", "一人", "個人", "単独",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"協力", "サポート", "助け合い", "連携"},
					ScoreChange: 3,
					Description: "チームワークの経験あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"まとめ役", "サポート役", "アイデア出し", "役割"},
					ScoreChange: 2,
					Description: "役割意識がある",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 2,
					Description: "具体的なエピソードあり",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーがチーム活動について思い出せないようです。授業のグループワークやアルバイトでの協力経験など、もっと具体的に聞いてください。",
					Purpose:  "チーム経験の深掘り",
				},
			}),
			IsActive: true,
		},

		// リーダーシップ志向の質問
		{
			Category:      "リーダーシップ志向",
			QuestionText:  "グループで何かをする時、自分から提案したり、まとめ役をしたことはありますか？どんな小さなことでも構いません。",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"提案", "まとめ", "リーダー", "率先", "主導", "指示", "決定",
				"責任", "先導", "引っ張る",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "特にない", "苦手", "できない",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"提案", "主導", "率先", "リーダー"},
					ScoreChange: 3,
					Description: "リーダーシップ経験あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"難しかった", "大変だった", "工夫した", "乗り越えた"},
					ScoreChange: 2,
					Description: "困難を乗り越えた経験",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーがリーダー経験について思い出せないようです。サークルの役員、グループワークでの提案、友人との計画立案など、もっと身近な例で聞いてください。",
					Purpose:  "リーダーシップ経験の深掘り",
				},
			}),
			IsActive: true,
		},

		// 創造性志向の質問
		{
			Category:      "創造性志向",
			QuestionText:  "新しいアイデアを考えたり、今までにない方法で問題を解決したことはありますか？どんな工夫をしましたか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"アイデア", "工夫", "考えた", "発想", "創造", "新しい", "独自",
				"オリジナル", "ユニーク", "改善",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "特にない", "思いつかない",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"アイデア", "工夫", "考えた", "発想"},
					ScoreChange: 3,
					Description: "創造的思考の経験あり",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 2,
					Description: "具体例を含んでいる",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーが創造性について思い出せないようです。趣味での工夫、勉強方法の改善、イベントの企画など、もっと身近な例で聞いてください。",
					Purpose:  "創造性の深掘り",
				},
			}),
			IsActive: true,
		},

		// 安定志向の質問
		{
			Category:      "安定志向",
			QuestionText:  "将来のキャリアについて、安定して長く働ける環境と、チャレンジングだけど変化が多い環境、どちらに魅力を感じますか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"安定", "長期", "継続", "福利厚生", "保険", "退職金",
				"ワークライフバランス", "定着", "腰を据えて",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"安定", "長期", "継続", "長く"},
					ScoreChange: 3,
					Description: "安定志向",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"福利厚生", "保険", "退職金", "ワークライフバランス"},
					ScoreChange: 2,
					Description: "福利厚生重視",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{}),
			IsActive:      true,
		},

		// 成長志向の質問
		{
			Category:      "成長志向",
			QuestionText:  "新しいスキルを学んだり、自分を成長させることは好きですか？最近、何か新しいことに挑戦しましたか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"学習", "成長", "挑戦", "スキル", "資格", "勉強", "自己啓発",
				"向上", "習得", "研鑽",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "特にない", "苦手",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"学習", "成長", "挑戦", "スキル"},
					ScoreChange: 3,
					Description: "成長意欲あり",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 2,
					Description: "具体的な学習経験",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "high_score",
					UseAI:    true,
					AIPrompt: "ユーザーは成長意欲が高いようです。その学習をどのように続けているか、モチベーションは何かを聞いてください。",
					Purpose:  "成長意欲の確認",
				},
			}),
			IsActive: true,
		},

		// ワークライフバランスの質問
		{
			Category:      "ワークライフバランス",
			QuestionText:  "仕事とプライベートのバランスについてどう考えていますか？仕事以外の時間も大切にしたいですか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"バランス", "プライベート", "趣味", "家族", "友人", "休日",
				"リフレッシュ", "余暇", "自分の時間", "オフ", "健康",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "どちらでも",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"バランス", "プライベート", "趣味", "大切"},
					ScoreChange: 3,
					Description: "ワークライフバランス重視",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"家族", "友人", "健康", "リフレッシュ"},
					ScoreChange: 2,
					Description: "私生活を大切にする姿勢",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 1,
					Description: "具体的な考えがある",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "high_score",
					UseAI:    true,
					AIPrompt: "ユーザーはワークライフバランスを重視しているようです。プライベートの時間で何をしたいか、どう過ごしたいかを聞いてください。",
					Purpose:  "ライフスタイルの確認",
				},
			}),
			IsActive: true,
		},

		// チャレンジ志向の質問
		{
			Category:      "チャレンジ志向",
			QuestionText:  "難しい課題や新しいことに挑戦するのは好きですか？失敗を恐れずにトライすることができますか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"挑戦", "チャレンジ", "トライ", "新しい", "難しい", "やってみる",
				"失敗", "経験", "冒険", "果敢", "積極的",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "怖い", "不安", "嫌い", "苦手",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"挑戦", "チャレンジ", "トライ", "やってみる"},
					ScoreChange: 3,
					Description: "チャレンジ精神あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"失敗", "学んだ", "成長", "経験"},
					ScoreChange: 2,
					Description: "失敗から学ぶ姿勢",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 2,
					Description: "具体的な挑戦経験あり",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーがチャレンジについて答えられなかったようです。今までに初めてやったこと、少し勇気が必要だったことなど、小さな挑戦でも良いので聞いてください。",
					Purpose:  "チャレンジ経験の深掘り",
				},
				{
					Trigger:  "high_score",
					UseAI:    true,
					AIPrompt: "ユーザーはチャレンジ精神が旺盛なようです。その挑戦で最も困難だったことや、どう乗り越えたかを聞いてください。",
					Purpose:  "チャレンジ力の確認",
				},
			}),
			IsActive: true,
		},

		// 細部志向の質問
		{
			Category:      "細部志向",
			QuestionText:  "物事を進めるとき、細かいところまで気を配るタイプですか？それとも、全体像を重視するタイプですか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"細かい", "丁寧", "正確", "確認", "チェック", "詳細", "気を配る",
				"ミス", "品質", "完璧", "慎重",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "適当", "大雑把",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"細かい", "丁寧", "正確", "確認"},
					ScoreChange: 3,
					Description: "細部への注意力あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"チェック", "ミス", "品質", "慎重"},
					ScoreChange: 2,
					Description: "品質意識が高い",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 1,
					Description: "具体例あり",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーが自分のスタイルについて答えられなかったようです。課題やレポートを書くとき、見直しや確認をするか、具体的な作業の進め方を聞いてください。",
					Purpose:  "作業スタイルの確認",
				},
			}),
			IsActive: true,
		},

		// コミュニケーション力の質問
		{
			Category:      "コミュニケーション力",
			QuestionText:  "人と話すことや、自分の考えを伝えることは得意ですか？どんな場面でコミュニケーションを取ることが多いですか？",
			TargetLevel:   "新卒",
			Priority:      10,
			AllowedPhases: defaultPhases,
			PositiveKeywords: mustMarshalJSON([]string{
				"話す", "伝える", "コミュニケーション", "説明", "相談", "対話",
				"プレゼン", "発表", "交流", "議論", "聞く", "理解",
			}),
			NegativeKeywords: mustMarshalJSON([]string{
				"わからない", "苦手", "嫌い", "できない",
			}),
			ScoreRules: mustMarshalJSON([]ScoreRule{
				{
					Condition:   "contains_any",
					Keywords:    []string{"話す", "伝える", "コミュニケーション", "説明"},
					ScoreChange: 3,
					Description: "コミュニケーション能力あり",
				},
				{
					Condition:   "contains_any",
					Keywords:    []string{"プレゼン", "発表", "議論", "相談"},
					ScoreChange: 2,
					Description: "積極的なコミュニケーション",
				},
				{
					Condition:   "has_example",
					Keywords:    []string{},
					ScoreChange: 2,
					Description: "具体的な経験あり",
				},
			}),
			FollowUpRules: mustMarshalJSON([]FollowUpRule{
				{
					Trigger:  "low_confidence",
					UseAI:    true,
					AIPrompt: "ユーザーがコミュニケーションについて答えられなかったようです。友人との会話、授業での発言、アルバイトでの接客など、日常的な場面で聞いてください。",
					Purpose:  "コミュニケーション経験の深掘り",
				},
				{
					Trigger:  "high_score",
					UseAI:    true,
					AIPrompt: "ユーザーはコミュニケーション力が高いようです。相手に分かりやすく伝えるために工夫していることを聞いてください。",
					Purpose:  "コミュニケーションスキルの確認",
				},
			}),
			IsActive: true,
		},
	}

	for _, q := range questions {
		if err := db.Create(&q).Error; err != nil {
			return err
		}
	}

	println("Predefined questions seeded successfully")
	return nil
}

// mustMarshalJSON JSONにマーシャル（エラー時はパニック）
func mustMarshalJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}
