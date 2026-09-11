package main

import (
	"Backend/internal/config"
	"Backend/internal/models"
	"log"
)

func main() {
	// 設定を読み込む（環境変数の読み込みはconfig.LoadConfig内で実施）
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// データベース接続
	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// サンプルデータ作成
	log.Println("Creating sample questions...")

	questions := []models.QuestionWeight{
		{
			Question:       "プログラミングやシステム開発に興味はありますか？",
			WeightCategory: "技術志向",
			WeightValue:    8,
			Description:    "技術的な仕事への興味を判定",
			IsActive:       true,
		},
		{
			Question:       "チームでのプロジェクト経験について教えてください",
			WeightCategory: "チームワーク志向",
			WeightValue:    7,
			Description:    "協調性とチームワーク能力を判定",
			IsActive:       true,
		},
		{
			Question:       "リーダーシップを発揮した経験はありますか？",
			WeightCategory: "リーダーシップ志向",
			WeightValue:    9,
			Description:    "リーダーシップ能力を判定",
			IsActive:       true,
		},
		{
			Question:       "新しいアイデアを考えることは得意ですか？",
			WeightCategory: "創造性志向",
			WeightValue:    6,
			Description:    "創造性と発想力を判定",
			IsActive:       true,
		},
		{
			Question:       "データ分析や論理的思考は得意ですか？",
			WeightCategory: "技術志向",
			WeightValue:    7,
			Description:    "分析力と論理的思考力を判定",
			IsActive:       true,
		},
		{
			Question:       "人とコミュニケーションを取ることは好きですか？",
			WeightCategory: "コミュニケーション力",
			WeightValue:    8,
			Description:    "コミュニケーション能力を判定",
			IsActive:       true,
		},
		{
			Question:       "問題解決のために技術的なアプローチを取ることはありますか？",
			WeightCategory: "技術志向",
			WeightValue:    7,
			Description:    "技術的な問題解決能力を判定",
			IsActive:       true,
		},
		{
			Question:       "複数の人と協力して何かを成し遂げた経験を教えてください",
			WeightCategory: "チームワーク志向",
			WeightValue:    8,
			Description:    "チームでの成果創出能力を判定",
			IsActive:       true,
		},
		{
			Question:       "プロジェクトを主導した経験について教えてください",
			WeightCategory: "リーダーシップ志向",
			WeightValue:    8,
			Description:    "プロジェクト主導力を判定",
			IsActive:       true,
		},
		{
			Question:       "既存のやり方を改善した経験はありますか？",
			WeightCategory: "創造性志向",
			WeightValue:    7,
			Description:    "改善力と創意工夫を判定",
			IsActive:       true,
		},
	}

	var successCount, duplicateCount int
	for _, q := range questions {
		// 重複チェック
		var count int64
		db.Model(&models.QuestionWeight{}).
			Where("question = ? AND weight_category = ?", q.Question, q.WeightCategory).
			Count(&count)

		if count > 0 {
			log.Printf("Skipping duplicate: %s (%s)", q.Question, q.WeightCategory)
			duplicateCount++
			continue
		}

		if err := db.Create(&q).Error; err != nil {
			log.Printf("Failed to create question: %v", err)
			continue
		}
		successCount++
		log.Printf("Created: %s (%s)", q.Question, q.WeightCategory)
	}

	log.Printf("\nSample data creation completed!")
	log.Printf("Success: %d, Duplicates: %d, Total: %d", successCount, duplicateCount, len(questions))
}
