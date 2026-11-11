// ローカル統合テスト環境（compose.test.yml）専用のシード。
// 固定の学生ユーザー1件を冪等に作成する（本番/stagingでは実行しないこと）。
// 実ログイン（POST /api/auth/login）で使うため、実際の bcrypt ハッシュ・
// メール認証済み状態など Login() が要求する条件をすべて満たす形で作る。
package main

import (
	"Backend/internal/config"
	"Backend/internal/models"
	"errors"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// E2ETestStudentEmail / E2ETestStudentPassword はローカル統合テストの
// smoke spec（frontend/e2e/smoke/smoke.spec.ts）と値を合わせる固定の認証情報。
// 本番データには存在しないため、固定値をコードに置いても問題ない。
const (
	E2ETestStudentEmail    = "e2e-student@example.com"
	E2ETestStudentPassword = "e2e-test-password-local-only"
	e2eBcryptCost          = 12 // 本番の bcryptCost (auth_service.go) と同じ
)

func main() {
	// 固定パスワードのユーザーを書き込むため、接続先は必ずテスト用DBに固定する。
	// Load は既存の環境変数を上書きしない（個人devの DB_* がexport済みだと誤接続する）ので
	// Overload を使う。読めなければ個人の Backend/.env へフォールバックさせず終了する。
	if err := godotenv.Overload(".env.test"); err != nil {
		log.Fatalf(".env.test を読み込めません（%v）。'cp Backend/.env.test.example Backend/.env.test' を実行してください", err)
	}
	if host := os.Getenv("DB_HOST"); host != "127.0.0.1" && host != "localhost" {
		log.Fatalf("DB_HOST=%q はローカルではありません。テスト用DB以外へは書き込みません", host)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := config.ConnectDB(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(E2ETestStudentPassword), e2eBcryptCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}
	now := time.Now()

	var user models.User
	result := db.Where("email = ?", E2ETestStudentEmail).First(&user)
	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		log.Fatalf("Failed to look up e2e student user: %v", result.Error)
	}
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		user = models.User{
			OrganizationID:  1,
			Email:           E2ETestStudentEmail,
			Password:        string(hashed),
			Name:            "E2Eテスト学生",
			IsGuest:         false,
			Role:            "student",
			EmailVerifiedAt: &now,
		}
		if err := db.Create(&user).Error; err != nil {
			log.Fatalf("Failed to create e2e student user: %v", err)
		}
		log.Printf("Created e2e student user: %s (id=%d)", E2ETestStudentEmail, user.ID)
		return
	}

	// 既に存在する場合はログインに必要な状態へ揃える（冪等）。
	user.Password = string(hashed)
	user.IsGuest = false
	user.WithdrawnAt = nil
	user.EmailVerifiedAt = &now
	user.LastLoginAt = nil
	if err := db.Save(&user).Error; err != nil {
		log.Fatalf("Failed to reset e2e student user: %v", err)
	}
	log.Printf("Reset e2e student user: %s (id=%d)", E2ETestStudentEmail, user.ID)
}
