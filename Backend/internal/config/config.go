package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBUser           string
	DBPass           string
	DBHost           string
	DBPort           string
	DBName           string
	ServerPort       string
	GBizInfoBaseURL  string
	GBizInfoToken    string
	AdminSecret      string
	UserSecret       string
	OAuthStateSecret string
}

func LoadConfig() (*Config, error) {
	env := os.Getenv("APP_ENV")

	if env != "production" {
		// ローカル開発環境では .env ファイルを読み込む
		err := godotenv.Load()
		if err != nil {
			log.Println("Warning: .env file not found. Skipping.")
		}
	}

	adminSecret := os.Getenv("ADMIN_SECRET")
	userSecret := os.Getenv("USER_SECRET")
	oauthStateSecret := os.Getenv("OAUTH_STATE_SECRET")

	if env == "production" {
		// 本番環境: シークレット未設定・プレースホルダーのまま起動禁止
		placeholders := map[string]string{
			"ADMIN_SECRET":      "change-me-admin-secret",
			"USER_SECRET":       "change-me-user-secret",
			"OAUTH_STATE_SECRET": "change-me-oauth-state-secret",
		}
		vals := map[string]string{
			"ADMIN_SECRET":      adminSecret,
			"USER_SECRET":       userSecret,
			"OAUTH_STATE_SECRET": oauthStateSecret,
		}
		for key, val := range vals {
			if val == "" || val == placeholders[key] {
				log.Fatalf("%s が未設定またはプレースホルダーのままです。本番環境では必ず安全な値を設定してください。", key)
			}
		}
	} else {
		if adminSecret == "" {
			log.Println("WARNING: ADMIN_SECRET が設定されていません。管理者認証トークンの検証が無効化されます。本番環境では必ず設定してください。")
		}
		if userSecret == "" {
			log.Println("WARNING: USER_SECRET が設定されていません。ユーザー認証が利用できません。本番環境では必ず設定してください。")
		}
		if oauthStateSecret == "" {
			log.Fatal("OAUTH_STATE_SECRET が設定されていません。OAuth CSRF 対策が機能しないため起動を中止します。.env に OAUTH_STATE_SECRET を設定してください。")
		}
	}

	cfg := &Config{
		DBUser:           os.Getenv("DB_USER"),
		DBPass:           os.Getenv("DB_PASSWORD"),
		DBHost:           os.Getenv("DB_HOST"),
		DBPort:           os.Getenv("DB_PORT"),
		DBName:           os.Getenv("DB_NAME"),
		ServerPort:       get("SERVER_PORT", "80"),
		GBizInfoBaseURL:  get("GBIZINFO_BASE_URL", ""),
		GBizInfoToken:    getFirst("GBIZINFO_API_KEY", "GBIZINFO_API_TOKEN"),
		AdminSecret:      adminSecret,
		UserSecret:       userSecret,
		OAuthStateSecret: oauthStateSecret,
	}

	// 必須値チェック
	if cfg.DBHost == "" || cfg.DBPort == "" || cfg.DBUser == "" || cfg.DBPass == "" || cfg.DBName == "" {
		log.Fatal("Missing one or more required environment variables for database connection")
	}

	return cfg, nil
}

// DSN は mysql ドライバ用の接続文字列を返す（例: user:pass@tcp(host:port)/dbname?parseTime=true&charset=utf8mb4&loc=Local）
func (c *Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		c.DBUser, c.DBPass, c.DBHost, c.DBPort, c.DBName)
}

func get(key, def string) string {
	value := os.Getenv(key)
	if value == "" {
		return def
	}
	return value
}

// getFirst returns the first non-empty value among the given env var names.
func getFirst(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}
