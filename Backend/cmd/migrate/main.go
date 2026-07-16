// cmd/migrate はバージョン管理型DBマイグレーションのCLI (#614)
//
// Usage:
//
//	go run ./cmd/migrate            # up: 未適用のマイグレーションをすべて適用（デフォルト）
//	go run ./cmd/migrate up         # 同上
//	go run ./cmd/migrate down       # 直近のマイグレーションを1つロールバック
//	go run ./cmd/migrate version    # 現在のバージョンと dirty フラグを表示
//	go run ./cmd/migrate force <N>  # バージョンを強制設定（dirty 状態からの復旧用）
//
// SEED_DATA=true を指定すると up 実行後に初期データを投入する。
package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"Backend/internal/models"
	"Backend/migrations"
)

func main() {
	loadEnv()
	dsn := buildDSN()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		log.Println("Applying migrations...")
		if err := migrations.Up(dsn); err != nil {
			log.Fatalf("Failed to apply migrations: %v", err)
		}
		log.Println("✓ Migrations applied successfully")
		printVersion(dsn)

		if os.Getenv("SEED_DATA") == "true" {
			if err := runSeed(dsn); err != nil {
				log.Fatalf("Failed to seed data: %v", err)
			}
			log.Println("✓ Data seeding completed successfully")
		}

	case "down":
		log.Println("Rolling back last migration...")
		if err := migrations.Down(dsn); err != nil {
			log.Fatalf("Failed to rollback: %v", err)
		}
		log.Println("✓ Rollback completed successfully")
		printVersion(dsn)

	case "version":
		printVersion(dsn)

	case "force":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate force <version>")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Invalid version %q: %v", os.Args[2], err)
		}
		if err := migrations.Force(dsn, v); err != nil {
			log.Fatalf("Failed to force version: %v", err)
		}
		log.Printf("✓ Forced version to %d", v)

	default:
		log.Fatalf("Unknown command %q (available: up, down, version, force)", cmd)
	}
}

func loadEnv() {
	if os.Getenv("APP_ENV") == "production" {
		return
	}
	// ローカル開発環境では .env ファイルを読み込む
	for _, envPath := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("Loaded .env file from: %s", envPath)
			return
		}
	}
	log.Println("Warning: .env file not found. Skipping.")
}

func buildDSN() string {
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "3306")
	user := os.Getenv("DB_USER")
	if user == "" {
		log.Fatal("DB_USER is required")
	}
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		log.Fatal("DB_NAME is required")
	}

	log.Printf("Target database: %s@tcp(%s:%s)/%s", user, host, port, dbname)
	return fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname,
	)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func printVersion(dsn string) {
	version, dirty, err := migrations.Version(dsn)
	if err != nil {
		log.Fatalf("Failed to get version: %v", err)
	}
	log.Printf("Current version: %d (dirty: %v)", version, dirty)
}

// runSeed は初期データを投入する（スキーマ変更は行わない）
func runSeed(dsn string) error {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("シード用DB接続に失敗: %w", err)
	}

	log.Println("Seeding initial data...")
	if err := seedData(db); err != nil {
		return err
	}
	// SeedData の中で SeedPredefinedQuestions も呼ばれる
	return models.SeedData(db)
}

func seedData(db *gorm.DB) error {
	// AIテンプレートの初期データ
	templates := []models.AIQuestionTemplate{
		{
			Category:    "basic",
			Prompt:      "ユーザーの基本的な興味や価値観を探る質問を生成してください",
			BaseWeight:  8,
			ContextKeys: `["industry_ids", "job_category_ids"]`,
			IsActive:    true,
		},
		{
			Category:    "skill",
			Prompt:      "ユーザーの具体的なスキルや経験を深掘りする質問を生成してください",
			BaseWeight:  6,
			ContextKeys: `["answer_history"]`,
			IsActive:    true,
		},
		{
			Category:    "preference",
			Prompt:      "ユーザーの働き方や環境の希望を確認する質問を生成してください",
			BaseWeight:  5,
			ContextKeys: `["industry_ids", "job_category_ids", "answer_history"]`,
			IsActive:    true,
		},
	}

	for _, template := range templates {
		if err := db.FirstOrCreate(&template, models.AIQuestionTemplate{Category: template.Category}).Error; err != nil {
			return err
		}
	}

	// 重みルールの初期データ
	rules := []models.WeightRule{
		{
			Name:        "深掘りフェーズでの重み増加",
			Condition:   `{"phase": "deep"}`,
			WeightBoost: 2,
			Priority:    10,
			IsActive:    true,
		},
		{
			Name:        "初回の業界質問",
			Condition:   `{"industry_count": 0}`,
			WeightBoost: 3,
			Priority:    20,
			IsActive:    true,
		},
	}

	for _, rule := range rules {
		if err := db.FirstOrCreate(&rule, models.WeightRule{Name: rule.Name}).Error; err != nil {
			return err
		}
	}

	return nil
}
