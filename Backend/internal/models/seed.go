package models

import "gorm.io/gorm"

// SeedData データベースに初期データを投入
func SeedData(db *gorm.DB) error {
	// 業界データ
	if err := seedIndustries(db); err != nil {
		return err
	}

	// 職種データ
	if err := seedJobCategories(db); err != nil {
		return err
	}

	// 業界-職種の関連付け
	if err := seedIndustryJobCategories(db); err != nil {
		return err
	}

	// 重み付けルール
	if err := seedWeightRules(db); err != nil {
		return err
	}

	// AI質問テンプレート
	if err := seedAIQuestionTemplates(db); err != nil {
		return err
	}

	// 詳細な質問データ
	if err := seedDetailedQuestions(db); err != nil {
		return err
	}

	// 分析フェーズデータ
	if err := seedAnalysisPhases(db); err != nil {
		return err
	}

	// 事前定義質問データ（ルールベース判定用）
	if err := SeedPredefinedQuestions(db); err != nil {
		return err
	}

	// 過去にシードしたデモ企業データを削除
	if err := CleanupDemoCompanies(db); err != nil {
		return err
	}

	return nil
}

func seedIndustries(db *gorm.DB) error {
	var count int64
	db.Model(&Industry{}).Count(&count)
	if count > 0 {
		return nil // 既にデータがある場合はスキップ
	}

	industries := []Industry{
		{Code: "IT", Name: "情報通信業", NameEn: "Information Technology", Level: 0, DisplayOrder: 1, IsActive: true},
		{Code: "IT-SW", Name: "ソフトウェア開発", NameEn: "Software Development", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "IT-WEB", Name: "Web サービス", NameEn: "Web Services", Level: 1, DisplayOrder: 2, IsActive: true},
		{Code: "MFG", Name: "製造業", NameEn: "Manufacturing", Level: 0, DisplayOrder: 2, IsActive: true},
		{Code: "MFG-AUTO", Name: "自動車製造", NameEn: "Automotive", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "MFG-ELEC", Name: "電子機器製造", NameEn: "Electronics", Level: 1, DisplayOrder: 2, IsActive: true},
		{Code: "FIN", Name: "金融・保険業", NameEn: "Finance & Insurance", Level: 0, DisplayOrder: 3, IsActive: true},
		{Code: "FIN-BANK", Name: "銀行", NameEn: "Banking", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "FIN-INS", Name: "保険", NameEn: "Insurance", Level: 1, DisplayOrder: 2, IsActive: true},
		{Code: "CONS", Name: "コンサルティング", NameEn: "Consulting", Level: 0, DisplayOrder: 4, IsActive: true},
		{Code: "EDU", Name: "教育・学習支援業", NameEn: "Education", Level: 0, DisplayOrder: 5, IsActive: true},
		{Code: "MED", Name: "医療・福祉", NameEn: "Healthcare", Level: 0, DisplayOrder: 6, IsActive: true},
	}

	return db.Create(&industries).Error
}

func seedJobCategories(db *gorm.DB) error {
	var count int64
	db.Model(&JobCategory{}).Count(&count)
	if count > 0 {
		return nil
	}

	jobCategories := []JobCategory{
		{Code: "ENG", Name: "エンジニア", NameEn: "Engineer", Level: 0, DisplayOrder: 1, IsActive: true},
		{Code: "ENG-SW", Name: "ソフトウェアエンジニア", NameEn: "Software Engineer", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "ENG-WEB", Name: "Webエンジニア", NameEn: "Web Engineer", Level: 1, DisplayOrder: 2, IsActive: true},
		{Code: "ENG-DATA", Name: "データエンジニア", NameEn: "Data Engineer", Level: 1, DisplayOrder: 3, IsActive: true},
		{Code: "SALES", Name: "営業", NameEn: "Sales", Level: 0, DisplayOrder: 2, IsActive: true},
		{Code: "SALES-B2B", Name: "法人営業", NameEn: "B2B Sales", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "SALES-B2C", Name: "個人営業", NameEn: "B2C Sales", Level: 1, DisplayOrder: 2, IsActive: true},
		{Code: "MKT", Name: "マーケティング", NameEn: "Marketing", Level: 0, DisplayOrder: 3, IsActive: true},
		{Code: "MKT-DIG", Name: "デジタルマーケティング", NameEn: "Digital Marketing", Level: 1, DisplayOrder: 1, IsActive: true},
		{Code: "HR", Name: "人事", NameEn: "Human Resources", Level: 0, DisplayOrder: 4, IsActive: true},
		{Code: "FIN-ACC", Name: "財務・経理", NameEn: "Finance & Accounting", Level: 0, DisplayOrder: 5, IsActive: true},
		{Code: "CONS-BIZ", Name: "経営コンサルタント", NameEn: "Business Consultant", Level: 0, DisplayOrder: 6, IsActive: true},
	}

	return db.Create(&jobCategories).Error
}

func seedWeightRules(db *gorm.DB) error {
	var count int64
	db.Model(&WeightRule{}).Count(&count)
	if count > 0 {
		return nil
	}

	weightRules := []WeightRule{
		{
			Name:        "技術関連キーワード強化",
			Condition:   `{"keywords": ["プログラミング", "コード", "技術", "開発", "エンジニア"]}`,
			WeightBoost: 5,
			Description: "技術関連のキーワードが含まれる場合、技術志向のスコアを強化",
			Priority:    10,
			IsActive:    true,
		},
		{
			Name:        "チーム関連キーワード強化",
			Condition:   `{"keywords": ["チーム", "協力", "協働", "メンバー", "リーダー"]}`,
			WeightBoost: 3,
			Description: "チームワーク関連のキーワードが含まれる場合、チームワークのスコアを強化",
			Priority:    8,
			IsActive:    true,
		},
		{
			Name:        "コミュニケーション関連キーワード強化",
			Condition:   `{"keywords": ["コミュニケーション", "説明", "伝える", "話す", "対話"]}`,
			WeightBoost: 4,
			Description: "コミュニケーション関連のキーワードが含まれる場合、スコアを強化",
			Priority:    9,
			IsActive:    true,
		},
		{
			Name:        "分析思考キーワード強化",
			Condition:   `{"keywords": ["分析", "データ", "論理", "問題解決", "思考"]}`,
			WeightBoost: 4,
			Description: "分析思考関連のキーワードが含まれる場合、スコアを強化",
			Priority:    7,
			IsActive:    true,
		},
		{
			Name:        "創造性キーワード強化",
			Condition:   `{"keywords": ["創造", "アイデア", "革新", "新しい", "デザイン"]}`,
			WeightBoost: 3,
			Description: "創造性関連のキーワードが含まれる場合、スコアを強化",
			Priority:    6,
			IsActive:    true,
		},
		{
			Name:        "リーダーシップキーワード強化",
			Condition:   `{"keywords": ["リーダー", "リード", "指導", "マネジメント", "統率"]}`,
			WeightBoost: 5,
			Description: "リーダーシップ関連のキーワードが含まれる場合、スコアを強化",
			Priority:    8,
			IsActive:    true,
		},
	}

	return db.Create(&weightRules).Error
}

func seedAIQuestionTemplates(db *gorm.DB) error {
	var count int64
	db.Model(&AIQuestionTemplate{}).Count(&count)
	if count > 0 {
		return nil
	}

	templates := []AIQuestionTemplate{
		{
			Category:    "技術志向",
			Prompt:      "あなたの技術的な興味や経験について教えてください。特に最近取り組んでいることや学んでいることはありますか？",
			BaseWeight:  8,
			ContextKeys: `["技術", "開発", "プログラミング"]`,
			IsActive:    true,
		},
		{
			Category:    "コミュニケーション",
			Prompt:      "チームでの作業において、あなたはどのようにコミュニケーションを取りますか？具体的なエピソードがあれば教えてください。",
			BaseWeight:  7,
			ContextKeys: `["チーム", "コミュニケーション", "協力"]`,
			IsActive:    true,
		},
		{
			Category:    "チームワーク",
			Prompt:      "チームプロジェクトでの経験について教えてください。あなたはどのような役割を担当しましたか？",
			BaseWeight:  7,
			ContextKeys: `["チーム", "プロジェクト", "役割"]`,
			IsActive:    true,
		},
		{
			Category:    "リーダーシップ",
			Prompt:      "グループやチームを率いた経験はありますか？その時どのようなアプローチを取りましたか？",
			BaseWeight:  8,
			ContextKeys: `["リーダー", "リード", "指導"]`,
			IsActive:    true,
		},
		{
			Category:    "分析思考",
			Prompt:      "複雑な問題に直面したとき、あなたはどのように分析し解決しますか？具体例を教えてください。",
			BaseWeight:  8,
			ContextKeys: `["分析", "問題解決", "論理"]`,
			IsActive:    true,
		},
		{
			Category:    "創造性",
			Prompt:      "今までで最も創造的だと思うアイデアや解決策は何ですか？それはどのような場面で生まれましたか？",
			BaseWeight:  7,
			ContextKeys: `["創造", "アイデア", "革新"]`,
			IsActive:    true,
		},
	}

	return db.Create(&templates).Error
}

func seedIndustryJobCategories(db *gorm.DB) error {
	var count int64
	db.Model(&IndustryJobCategory{}).Count(&count)
	if count > 0 {
		return nil
	}

	// IT業界とエンジニア職の関連付け
	relations := []IndustryJobCategory{
		// IT業界
		{IndustryID: 1, JobCategoryID: 1, IsCommon: true, DisplayOrder: 1},  // IT - エンジニア
		{IndustryID: 1, JobCategoryID: 2, IsCommon: true, DisplayOrder: 2},  // IT - ソフトウェアエンジニア
		{IndustryID: 1, JobCategoryID: 3, IsCommon: true, DisplayOrder: 3},  // IT - Webエンジニア
		{IndustryID: 1, JobCategoryID: 4, IsCommon: true, DisplayOrder: 4},  // IT - データエンジニア
		{IndustryID: 1, JobCategoryID: 5, IsCommon: false, DisplayOrder: 5}, // IT - 営業
		{IndustryID: 1, JobCategoryID: 8, IsCommon: false, DisplayOrder: 6}, // IT - マーケティング

		// 製造業
		{IndustryID: 4, JobCategoryID: 1, IsCommon: true, DisplayOrder: 1},   // 製造業 - エンジニア
		{IndustryID: 4, JobCategoryID: 5, IsCommon: true, DisplayOrder: 2},   // 製造業 - 営業
		{IndustryID: 4, JobCategoryID: 10, IsCommon: false, DisplayOrder: 3}, // 製造業 - 人事

		// 金融・保険業
		{IndustryID: 7, JobCategoryID: 5, IsCommon: true, DisplayOrder: 1},   // 金融 - 営業
		{IndustryID: 7, JobCategoryID: 11, IsCommon: true, DisplayOrder: 2},  // 金融 - 財務・経理
		{IndustryID: 7, JobCategoryID: 12, IsCommon: false, DisplayOrder: 3}, // 金融 - コンサルタント

		// コンサルティング
		{IndustryID: 10, JobCategoryID: 12, IsCommon: true, DisplayOrder: 1}, // コンサルティング - コンサルタント
		{IndustryID: 10, JobCategoryID: 1, IsCommon: false, DisplayOrder: 2}, // コンサルティング - エンジニア
	}

	return db.Create(&relations).Error
}

func seedDetailedQuestions(db *gorm.DB) error {
	var count int64
	db.Model(&QuestionWeight{}).Count(&count)
	if count >= 30 {
		return nil // 既に詳細な質問がある場合はスキップ
	}

	questions := []QuestionWeight{
		// 技術志向
		{
			Question:       "プログラミングを学んだことはありますか？学んだきっかけや、どのような言語・技術に興味を持ちましたか？",
			WeightCategory: "技術志向",
			WeightValue:    8,
			Description:    "技術への興味と学習経験を評価",
			IsActive:       true,
		},
		{
			Question:       "技術的な課題や問題に直面したとき、どのようにアプローチしますか？具体的な例があれば教えてください。",
			WeightCategory: "技術志向",
			WeightValue:    9,
			Description:    "技術的問題解決へのアプローチを評価",
			IsActive:       true,
		},
		{
			Question:       "最近読んだ技術記事や興味を持った新技術はありますか？それについて簡単に説明してください。",
			WeightCategory: "技術志向",
			WeightValue:    7,
			Description:    "技術への継続的関心を評価",
			IsActive:       true,
		},

		// コミュニケーション能力
		{
			Question:       "グループディスカッションやミーティングで、自分の意見をどのように伝えていますか？",
			WeightCategory: "コミュニケーション能力",
			WeightValue:    8,
			Description:    "意見表明のスキルを評価",
			IsActive:       true,
		},
		{
			Question:       "相手に複雑なことを説明する必要があった経験はありますか？どのように工夫しましたか？",
			WeightCategory: "コミュニケーション能力",
			WeightValue:    9,
			Description:    "説明力と伝達スキルを評価",
			IsActive:       true,
		},
		{
			Question:       "意見が対立したとき、どのように相手の意見を聞き、理解しようとしますか？",
			WeightCategory: "コミュニケーション能力",
			WeightValue:    8,
			Description:    "傾聴力と対話能力を評価",
			IsActive:       true,
		},

		// リーダーシップ
		{
			Question:       "グループやチームで、自分から率先して動いたり、メンバーをまとめたりした経験はありますか？",
			WeightCategory: "リーダーシップ",
			WeightValue:    9,
			Description:    "主体性とリーダー経験を評価",
			IsActive:       true,
		},
		{
			Question:       "チームの目標達成のために、どのような工夫や働きかけをしたことがありますか？",
			WeightCategory: "リーダーシップ",
			WeightValue:    8,
			Description:    "目標達成への貢献度を評価",
			IsActive:       true,
		},
		{
			Question:       "メンバーのモチベーションが下がっているとき、どのように対応しますか？",
			WeightCategory: "リーダーシップ",
			WeightValue:    8,
			Description:    "メンバーサポート能力を評価",
			IsActive:       true,
		},

		// チームワーク
		{
			Question:       "チームプロジェクトで、あなたはどのような役割を担当することが多いですか？その理由は？",
			WeightCategory: "チームワーク",
			WeightValue:    7,
			Description:    "チーム内での役割認識を評価",
			IsActive:       true,
		},
		{
			Question:       "チームメンバーと協力して成果を出した経験について教えてください。",
			WeightCategory: "チームワーク",
			WeightValue:    8,
			Description:    "協働経験と成果を評価",
			IsActive:       true,
		},
		{
			Question:       "チーム内で苦手なメンバーがいた場合、どのように接しますか？",
			WeightCategory: "チームワーク",
			WeightValue:    7,
			Description:    "協調性と人間関係構築力を評価",
			IsActive:       true,
		},

		// 問題解決力
		{
			Question:       "複雑な課題に直面したとき、どのように問題を整理し、解決策を考えますか？",
			WeightCategory: "問題解決力",
			WeightValue:    9,
			Description:    "論理的思考と問題分析力を評価",
			IsActive:       true,
		},
		{
			Question:       "予期せぬトラブルが発生したとき、どのように対処しましたか？具体例を教えてください。",
			WeightCategory: "問題解決力",
			WeightValue:    8,
			Description:    "トラブル対応力を評価",
			IsActive:       true,
		},
		{
			Question:       "問題の原因を特定するために、どのような手順や方法を使いますか？",
			WeightCategory: "問題解決力",
			WeightValue:    8,
			Description:    "分析手法と論理性を評価",
			IsActive:       true,
		},

		// 創造性・発想力
		{
			Question:       "今までで最も創造的だと思うアイデアや提案は何ですか？どのように思いつきましたか？",
			WeightCategory: "創造性・発想力",
			WeightValue:    8,
			Description:    "アイデア創出力を評価",
			IsActive:       true,
		},
		{
			Question:       "既存のやり方ではうまくいかないとき、どのような新しいアプローチを試みますか？",
			WeightCategory: "創造性・発想力",
			WeightValue:    8,
			Description:    "柔軟な発想と挑戦姿勢を評価",
			IsActive:       true,
		},
		{
			Question:       "何か新しいことを始めたり、独自の工夫をしたりした経験はありますか？",
			WeightCategory: "創造性・発想力",
			WeightValue:    7,
			Description:    "革新性と独創性を評価",
			IsActive:       true,
		},

		// 計画性・実行力
		{
			Question:       "大きな目標を達成するために、どのように計画を立て、実行しますか？",
			WeightCategory: "計画性・実行力",
			WeightValue:    8,
			Description:    "計画立案と実行力を評価",
			IsActive:       true,
		},
		{
			Question:       "複数のタスクを同時に進める必要があるとき、どのように優先順位をつけますか？",
			WeightCategory: "計画性・実行力",
			WeightValue:    8,
			Description:    "タスク管理能力を評価",
			IsActive:       true,
		},
		{
			Question:       "計画通りに進まなかったとき、どのように対応しますか？",
			WeightCategory: "計画性・実行力",
			WeightValue:    7,
			Description:    "柔軟な対応力を評価",
			IsActive:       true,
		},

		// 学習意欲・成長志向
		{
			Question:       "最近、自分から進んで学んだことは何ですか？なぜそれを学ぼうと思いましたか？",
			WeightCategory: "学習意欲・成長志向",
			WeightValue:    9,
			Description:    "自主的学習姿勢を評価",
			IsActive:       true,
		},
		{
			Question:       "フィードバックや批判を受けたとき、どのように受け止め、活かしますか？",
			WeightCategory: "学習意欲・成長志向",
			WeightValue:    8,
			Description:    "成長マインドセットを評価",
			IsActive:       true,
		},
		{
			Question:       "失敗から学んだことや、それをどう次に活かしたかについて教えてください。",
			WeightCategory: "学習意欲・成長志向",
			WeightValue:    8,
			Description:    "失敗からの学習能力を評価",
			IsActive:       true,
		},

		// ストレス耐性・粘り強さ
		{
			Question:       "プレッシャーのかかる状況で、どのように自分を保ちますか？",
			WeightCategory: "ストレス耐性・粘り強さ",
			WeightValue:    8,
			Description:    "ストレス対処法を評価",
			IsActive:       true,
		},
		{
			Question:       "困難な状況でも諦めずに取り組んだ経験はありますか？何が原動力でしたか？",
			WeightCategory: "ストレス耐性・粘り強さ",
			WeightValue:    9,
			Description:    "粘り強さと動機づけを評価",
			IsActive:       true,
		},
		{
			Question:       "うまくいかないことが続いたとき、どのように気持ちを切り替えますか？",
			WeightCategory: "ストレス耐性・粘り強さ",
			WeightValue:    7,
			Description:    "レジリエンスを評価",
			IsActive:       true,
		},

		// ビジネス思考・目標志向
		{
			Question:       "仕事やプロジェクトにおいて、どのような成果を出すことを重視しますか？",
			WeightCategory: "ビジネス思考・目標志向",
			WeightValue:    8,
			Description:    "成果志向を評価",
			IsActive:       true,
		},
		{
			Question:       "顧客や利用者の視点で考えたり、行動したりした経験はありますか？",
			WeightCategory: "ビジネス思考・目標志向",
			WeightValue:    8,
			Description:    "顧客志向を評価",
			IsActive:       true,
		},
		{
			Question:       "将来、どのような価値を社会や組織に提供したいと考えていますか？",
			WeightCategory: "ビジネス思考・目標志向",
			WeightValue:    7,
			Description:    "キャリアビジョンと価値観を評価",
			IsActive:       true,
		},
	}

	return db.Create(&questions).Error
}

func seedAnalysisPhases(db *gorm.DB) error {
	var count int64
	db.Model(&AnalysisPhase{}).Count(&count)
	if count > 0 {
		updates := []AnalysisPhase{
			{PhaseName: "job_analysis", MinQuestions: 4, MaxQuestions: 0},
			{PhaseName: "interest_analysis", MinQuestions: 4, MaxQuestions: 0},
			{PhaseName: "aptitude_analysis", MinQuestions: 4, MaxQuestions: 0},
			{PhaseName: "future_analysis", MinQuestions: 3, MaxQuestions: 0},
		}
		for _, p := range updates {
			if err := db.Model(&AnalysisPhase{}).
				Where("phase_name = ?", p.PhaseName).
				Updates(map[string]any{
					"min_questions": p.MinQuestions,
					"max_questions": p.MaxQuestions,
				}).Error; err != nil {
				return err
			}
		}
		return nil
	}

	phases := []AnalysisPhase{
		{
			PhaseName:    "job_analysis",
			DisplayName:  "職種分析",
			PhaseOrder:   1,
			Description:  "ユーザーが興味を持つIT職種や分野を特定し、基本的な方向性を確認します。",
			MinQuestions: 4,
			MaxQuestions: 0,
		},
		{
			PhaseName:    "interest_analysis",
			DisplayName:  "興味分析",
			PhaseOrder:   2,
			Description:  "技術的な興味や学習スタイル、好きな作業内容などを深掘りします。",
			MinQuestions: 4,
			MaxQuestions: 0,
		},
		{
			PhaseName:    "aptitude_analysis",
			DisplayName:  "適性分析",
			PhaseOrder:   3,
			Description:  "コミュニケーション能力、チームワーク、問題解決力などの適性を評価します。",
			MinQuestions: 4,
			MaxQuestions: 0,
		},
		{
			PhaseName:    "future_analysis",
			DisplayName:  "将来分析",
			PhaseOrder:   4,
			Description:  "キャリアビジョンや将来の目標、働き方の希望などを確認します。",
			MinQuestions: 3,
			MaxQuestions: 0,
		},
	}

	return db.Create(&phases).Error
}
