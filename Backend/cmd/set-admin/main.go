package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	email := flag.String("email", "", "管理者に昇格するユーザーのメールアドレス")
	flag.Parse()

	if *email == "" {
		fmt.Fprintln(os.Stderr, "使い方: go run ./cmd/set-admin --email user@example.com")
		os.Exit(1)
	}

	loadEnv()

	db, err := connectDB()
	if err != nil {
		log.Fatalf("DB接続失敗: %v", err)
	}

	result := db.Exec("UPDATE users SET is_admin = true WHERE email = ?", *email)
	if result.Error != nil {
		log.Fatalf("更新失敗: %v", result.Error)
	}
	if result.RowsAffected == 0 {
		log.Fatalf("ユーザーが見つかりません: %s", *email)
	}

	fmt.Printf("✓ %s を管理者に昇格しました\n", *email)
}

func loadEnv() {
	if os.Getenv("APP_ENV") == "production" {
		return
	}
	candidates := []string{".env", "../.env", "../../.env"}
	for _, p := range candidates {
		abs, _ := filepath.Abs(p)
		if _, err := os.Stat(abs); err == nil {
			_ = godotenv.Load(abs)
			return
		}
	}
}

func connectDB() (*gorm.DB, error) {
	host := os.Getenv("DB_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("DB_PORT")
	if port == "" {
		port = "3306"
	}
	user := os.Getenv("DB_USER")
	if user == "" {
		log.Fatal("DB_USER が設定されていません")
	}
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")
	if dbname == "" {
		log.Fatal("DB_NAME が設定されていません")
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		user, password, host, port, dbname)
	return gorm.Open(mysql.Open(dsn), &gorm.Config{})
}
