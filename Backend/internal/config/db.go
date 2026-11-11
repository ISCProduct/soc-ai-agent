package config

import (
	"os"
	"strconv"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// 既定値は db.t4g.micro の max_connections（約85）に合わせる。本番は最大2タスクなので、
// 1タスク30本なら MySQL の 1040 エラーへ届く前にアプリ側で待たせられる。
const (
	defaultDBMaxOpenConns    = 30
	defaultDBMaxIdleConns    = 10
	defaultDBConnMaxLifetime = 5 * time.Minute
)

// ConnectDB データベースに接続
func ConnectDB(cfg *Config) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(positiveIntEnv("DB_MAX_OPEN_CONNS", defaultDBMaxOpenConns))
	sqlDB.SetMaxIdleConns(positiveIntEnv("DB_MAX_IDLE_CONNS", defaultDBMaxIdleConns))
	// RDS のフェイルオーバーや wait_timeout で切れた接続を持ち続けない。
	sqlDB.SetConnMaxLifetime(defaultDBConnMaxLifetime)
	return db, nil
}

func positiveIntEnv(key string, fallback int) int {
	value, err := strconv.Atoi(os.Getenv(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
